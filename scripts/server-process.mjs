import { spawn, spawnSync } from 'node:child_process';
import { once } from 'node:events';
import { openSync, closeSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';
import { root, legacy, workspace, testEnvironment } from './env.mjs';
import { fixtureKey } from './fixture-db.mjs';

function listeners(port) {
  return spawnSync('lsof',['-t',`-iTCP:${port}`,'-sTCP:LISTEN'],{encoding:'utf8'}).stdout.trim().split(/\s+/).filter(Boolean).map(Number);
}
export async function borrowWorkspacePort(port, allowed) {
  if (!listeners(port).length) return async()=>{};
  if (!allowed) throw new Error(`Port ${port} is occupied; use --borrow-workspace to suspend and restore the recognized workspace service`);
  const application={3334:'kaderisasi-admin-be-go',3333:'kaderisasi-web-be',3005:'kaderisasi-admin-fe',3000:'kaderisasi-web-fe'}[port];
  if(!application)throw new Error('Unallocated workspace port');
  const directory = resolve(workspace,application);
  const panes = spawnSync('tmux',['list-panes','-a','-F','#{pane_id}\t#{pane_pid}\t#{pane_current_path}\t#{session_name}:#{window_index}'],{encoding:'utf8'}).stdout.trim().split('\n').map(row=>row.split('\t'));
  const pane=panes.find(row=>row[2]===directory);
  if (!pane) throw new Error(`Port ${port} is not managed by the expected workspace pane`);
  const mode = spawnSync('tmux',['show-option','-v','-t',pane[3],'@kaderisasi_environment'],{encoding:'utf8'}).stdout.trim();
  if(port === 3334 && !['test','prod'].includes(mode))throw new Error('Cannot restore Go without a recorded workspace environment');
  const resumeCommand = port === 3334 ? `node scripts/run.mjs --environment=${mode} api` : 'npm run dev';
  for(const listener of listeners(port)) {
    let pid=listener,owned=false;
    for(let n=0;n<10&&pid>1;n++) { if(pid===Number(pane[1])){owned=true;break;} pid=Number(spawnSync('ps',['-p',String(pid),'-o','ppid='],{encoding:'utf8'}).stdout.trim()); }
    if(!owned)throw new Error(`Port ${port} listener is not descended from the workspace pane`);
  }
  spawnSync('tmux',['send-keys','-t',pane[0],'C-c']);
  for(let n=0;n<100&&listeners(port).length;n++)await delay(100);
  if(listeners(port).length)throw new Error('Workspace service did not stop');
  writeFileSync(resolve(root,`.artifacts/borrowed-${port}.json`),JSON.stringify({port,pane:pane[0],target:pane[3],directory,status:'suspended'}));
  return async()=>{
    if(listeners(port).length)throw new Error(`Cannot restore occupied port ${port}`);
    const live=spawnSync('tmux',['list-panes','-a','-F','#{pane_id}'],{encoding:'utf8'}).stdout.trim().split('\n');
    if(live.includes(pane[0])) {
      spawnSync('tmux',['send-keys','-t',pane[0],'-l',`cd '${directory}' && ${resumeCommand}`]);
      spawnSync('tmux',['send-keys','-t',pane[0],'Enter']);
    } else {
      // The original launcher may close a pane when npm receives Ctrl-C.
      const replacement=spawnSync('tmux',['split-window','-d','-t',pane[3],'-c',directory,'-P','-F','#{pane_id}',`${resumeCommand}; exec /bin/zsh -l`],{encoding:'utf8'});
      if(replacement.status!==0)throw new Error('Failed to restore workspace pane');
      pane[0]=replacement.stdout.trim();
      spawnSync('tmux',['select-layout','-t',pane[3],'tiled']);
    }
    for(let n=0;n<150&&!listeners(port).length;n++)await delay(100);
    if(!listeners(port).length)throw new Error(`Workspace service on ${port} did not restart`);
    writeFileSync(resolve(root,`.artifacts/borrowed-${port}.json`),JSON.stringify({port,pane:pane[0],target:pane[3],directory,status:'restored'}));
  };
}

export async function startAdminFrontend(environment) {
  if(listeners(3005).length)throw new Error('Port 3005 must be free');
  const fd=openSync(resolve(root,'.artifacts/browser-admin-fe.log'),'w',0o600);
  const child=spawn('node',['node_modules/vite/bin/vite.js','--port','3005','--strictPort','--host','127.0.0.1'],{cwd:resolve(workspace,'kaderisasi-admin-fe'),env:{...environment,VITE_PUBLIC_BE_ADMIN_API:'http://localhost:3334/v2'},stdio:['ignore',fd,fd],detached:true});
  closeSync(fd);
  let ended=false;child.once('exit',()=>{ended=true});
  async function stop(){if(!ended){const done=once(child,'exit');process.kill(-child.pid,'SIGTERM');await Promise.race([done,delay(15000,undefined,{ref:false})]);if(!ended){process.kill(-child.pid,'SIGKILL');await done;}}}
  try{
    for(let n=0;n<150;n++){
      if(ended)throw new Error('Admin frontend exited; see .artifacts/browser-admin-fe.log');
      try{const response=await fetch('http://127.0.0.1:3005',{signal:AbortSignal.timeout(500)});if(response.ok)return{child,stop};}catch{}
      await delay(100);
    }
    throw new Error('Admin frontend startup timed out');
  }catch(error){await stop();throw error;}
}

export async function startServer(kind,schema,label=kind,options={}) {
  if(listeners(3334).length)throw new Error('Port 3334 must be free before sequential comparison');
  const logPath=resolve(root,`.artifacts/${label}-server.log`),fd=openSync(logPath,'w',0o600);
  const env=testEnvironment({NODE_ENV:'test',TZ:options.timezone??'Asia/Jakarta',PORT:'3334',HOST:'127.0.0.1',APP_KEY:fixtureKey,DB_SCHEMA:schema,PGOPTIONS:`-c search_path=${schema}`,GOOGLE_CLIENT_ID:'synthetic-client',ADMIN_CORS_ORIGINS:'http://localhost:3005',LOG_LEVEL:'error'});
  if(options.journal)env.GO_REWRITE_STORAGE_LEDGER=options.journal;
  if(options.googleKeys)env.GO_REWRITE_GOOGLE_KEYS_URL=options.googleKeys;
  if(options.origins)env.ADMIN_CORS_ORIGINS=options.origins;
  if(options.productionContract){env.NODE_ENV='production';env.GO_REWRITE_PRODUCTION_CONTRACT='1';}
  const child=kind==='adonis'
    ? spawn('node',[...(options.journal?['--require',resolve(root,'scripts/storage-instrument.cjs')]:[]),...(options.googleKeys?['--require',resolve(root,'scripts/google-instrument.cjs')]:[]),...(options.compiled?[]:['--import=ts-node-maintained/register/esm']),'--enable-source-maps','bin/server.js'],{cwd:options.compiled?resolve(legacy,'build'):legacy,env,stdio:['ignore',fd,fd],detached:true})
    : spawn(resolve(root,'.artifacts/admin-api'),[],{cwd:root,env,stdio:['ignore',fd,fd],detached:true});
  closeSync(fd);
  let ended=false;child.once('exit',()=>{ended=true});
  async function stop() {
    if(!ended){const complete=once(child,'exit');process.kill(-child.pid,'SIGTERM');await Promise.race([complete,delay(15000,undefined,{ref:false})]);if(!ended){process.kill(-child.pid,'SIGKILL');await complete;}}
  }
  try {
    for(let n=0;n<150;n++) {
      if(ended)throw new Error(`${kind} exited during startup; see ${logPath}`);
      try{const res=await fetch('http://127.0.0.1:3334/health',{signal:AbortSignal.timeout(300)});if(res.ok)return {child,stop,logPath};}catch{}
      await delay(100);
    }
    throw new Error(`${kind} startup timeout; see ${logPath}`);
  }catch(error){await stop();throw error}
}

export async function startWebBackend(schema,label='shared-web-be') {
  if(listeners(3333).length)throw new Error('Port 3333 must be free');
  const logPath=resolve(root,`.artifacts/${label}.log`),fd=openSync(logPath,'w',0o600);
  const child=spawn('node',['--import=ts-node-maintained/register/esm','--enable-source-maps','bin/server.js'],{
    cwd:resolve(workspace,'kaderisasi-web-be'),
    env:testEnvironment({NODE_ENV:'test',TZ:'Asia/Jakarta',PORT:'3333',HOST:'127.0.0.1',APP_KEY:fixtureKey,PGOPTIONS:`-c search_path=${schema}`,LOG_LEVEL:'error'}),
    stdio:['ignore',fd,fd],detached:true,
  });
  closeSync(fd);
  let ended=false;child.once('exit',()=>{ended=true;});
  async function stop(){if(!ended){const done=once(child,'exit');process.kill(-child.pid,'SIGTERM');await Promise.race([done,delay(15000,undefined,{ref:false})]);if(!ended){process.kill(-child.pid,'SIGKILL');await done;}}}
  try{
    for(let n=0;n<150;n++){
      if(ended)throw new Error(`web-be exited; see ${logPath}`);
      try{if((await fetch('http://127.0.0.1:3333/health',{signal:AbortSignal.timeout(300)})).ok)return{child,stop,logPath};}catch{}
      await delay(100);
    }
    throw new Error(`web-be startup timed out; see ${logPath}`);
  }catch(error){await stop();throw error;}
}

export async function startPublicFrontend(environment) {
  if(listeners(3000).length)throw new Error('Port 3000 must be free');
  const directory=resolve(workspace,'kaderisasi-web-fe');
  const logPath=resolve(root,'.artifacts/browser-public-fe.log'),fd=openSync(logPath,'w',0o600);
  const env={...environment,NODE_ENV:'production',NEXT_PUBLIC_APP_ENV:'test',NEXT_PUBLIC_APP_URL:'http://localhost:3000',SERVER_BE_API:'http://127.0.0.1:3333/v2',SERVER_BE_ADMIN_API:'http://127.0.0.1:3334/v2',NEXT_PUBLIC_BE_API:'http://localhost:3333/v2',NEXT_PUBLIC_BE_ADMIN_API:'http://localhost:3334/v2'};
  let child,ended=true;
  async function stop(){if(!ended){const done=once(child,'exit');process.kill(-child.pid,'SIGTERM');await Promise.race([done,delay(15000,undefined,{ref:false})]);if(!ended){process.kill(-child.pid,'SIGKILL');await done;}}}
  function launch(args){ended=false;child=spawn('node',['node_modules/next/dist/bin/next',...args],{cwd:directory,env,stdio:['ignore',fd,fd],detached:true});child.once('exit',()=>{ended=true;});return child;}
  try{
    const build=launch(['build']);
    const [code]=await once(build,'exit');
    if(code!==0)throw new Error(`Public frontend build failed; see ${logPath}`);
    launch(['start','--port','3000','--hostname','127.0.0.1']);
    for(let n=0;n<150;n++){
      if(ended)throw new Error(`Public frontend exited; see ${logPath}`);
      try{if((await fetch('http://127.0.0.1:3000/login',{signal:AbortSignal.timeout(1000)})).ok)return{child,stop,logPath};}catch{}
      await delay(100);
    }
    throw new Error(`Public frontend startup timed out; see ${logPath}`);
  }catch(error){await stop();throw error;}finally{closeSync(fd);}
}
