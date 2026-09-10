import assert from 'node:assert/strict';

export async function templateCases(h){
  const path='/v2/certificate-templates/1';
  await h.call('template:invalid','POST','/v2/certificate-templates',{});
  await h.call('template:direct-publish','POST','/v2/certificate-templates',{name:'Invalid publish',status:'published'});
  await h.call('template:create','POST','/v2/certificate-templates',{name:'Certificate fixture'});
  await h.call('template:unready','POST',path+'/publish',{expectedVersion:1});
  await h.call('template:missing-version','PUT',path,{name:'Missing version'});
  const background=await h.upload('template:background',path+'/background');
  await h.inspectObject(background.data.asset_key,{width:640,height:480});
  const asset=await h.upload('template:asset',path+'/assets');
  const design={backgroundUrl:null,canvasWidth:800,canvasHeight:566,elements:[{id:'name',type:'variable-text',variable:'{{name}}',x:100,y:100,width:600,height:80},{id:'signature',type:'signature',imageUrl:asset.data.asset_key,x:100,y:200,width:100,height:100}]};
  await h.call('template:design','PUT',path,{expectedVersion:2,templateData:design});
  const copy=await h.call('template:duplicate','POST',path+'/duplicate');
  assert.notEqual(copy.data.background_image,background.data.asset_key);
  assert.notEqual(copy.data.template_data.elements[1].imageUrl,asset.data.asset_key);
  await h.inspectObject(copy.data.background_image,{width:640,height:480});
  await h.inspectObject(copy.data.template_data.elements[1].imageUrl,{width:640,height:480});
  await h.call('template:publish','POST',path+'/publish',{expectedVersion:3});
  await h.call('template:published-edit','PUT',path,{expectedVersion:4,name:'Forbidden edit'});
  await h.upload('template:published-upload',path+'/assets');
  await h.call('template:version-conflict','POST',path+'/archive',{expectedVersion:3});
  await h.call('template:archive','POST',path+'/archive',{expectedVersion:4});
  await h.call('template:republish','POST',path+'/publish',{expectedVersion:5});
  await h.call('template:activity','POST','/v2/activities',{name:'Certificate activity',certificate_template_id:1});
  await h.call('template:show','GET',path);
  await h.call('template:delete-in-use','DELETE',path);
  for(const query of ['status=published&view=summary','is_active=true&search=Certificate','status=draft','is_active=false','per_page=500'])await h.call('template:list:'+query,'GET','/v2/certificate-templates?'+query);
  const copyPath='/v2/certificate-templates/2';
  await h.call('template:clear-description','PUT',copyPath,{expectedVersion:1,description:'  '});
  const broken={...copy.data.template_data,elements:[{...copy.data.template_data.elements[1],imageUrl:'certificate/templates/2/assets/missing-00000000-0000-4000-8000-000000000000.webp'}]};
  await h.call('template:missing-source-design','PUT',copyPath,{expectedVersion:2,templateData:broken});
  await h.call('template:failed-copy','POST',copyPath+'/duplicate');
  await h.call('template:delete-copy','DELETE',copyPath);
  await h.call('template:missing','GET','/v2/certificate-templates/999999');
  await h.call('template:invalid-id','GET','/v2/certificate-templates/01');
  await h.call('template:missing-update','PUT','/v2/certificate-templates/999999',{expectedVersion:1,name:'Missing'});
  await h.call('template:missing-duplicate','POST','/v2/certificate-templates/999999/duplicate');
  await h.call('template:missing-publish','POST','/v2/certificate-templates/999999/publish',{expectedVersion:1});
  await h.call('template:missing-delete','DELETE','/v2/certificate-templates/999999');
  await h.upload('template:missing-upload','/v2/certificate-templates/999999/assets');
  await h.call('template:missing-file','POST',path+'/background',{});
  await templateBoundaryCases(h);
}

async function templateBoundaryCases(h){
  for(const id of ['bad','01','1.5','2147483648','9007199254740991','9007199254740992','%31']){
    const path='/v2/certificate-templates/'+id;
    for(const [method,suffix,body] of [['GET',''],['PUT','',{expectedVersion:999999,name:'Boundary'}],['POST','/publish',{expectedVersion:999999}],['POST','/archive',{expectedVersion:999999}],['POST','/duplicate'],['DELETE','']])await h.call(`template:identifier:${id}:${method}:${suffix}`,method,path+suffix,body);
    await h.upload('template:identifier-upload:'+id,path+'/assets');
  }
  for(const version of [2147483648,9007199254740991,1e30])await h.call('template:expected-version:'+version,'POST','/v2/certificate-templates/1/archive',{expectedVersion:version});
  for(const query of ['page=2147483648','page=1e30','page=Infinity','page=1.5&per_page=1.5','status=wrong&is_active=true','status=published&is_active=false','search=%20Certificate%20&view=summary'])await h.call('template:boundary-list:'+query,'GET','/v2/certificate-templates?'+query);
  const created=await h.call('template:typed-optionals','POST','/v2/certificate-templates',{name:'  Typed design  ',description:null,isActive:true,templateData:{backgroundUrl:null,canvasWidth:'1000',canvasHeight:700,elements:[{id:'complete',type:'static-text',name:'Label',x:0,y:0,width:400,height:80,content:'Text',fontSize:16,fontFamily:'Arial',color:'#000000',textAlign:'center',verticalAlign:'middle',fontWeight:'bold',fontStyle:'italic',textDecoration:'underline',lineHeight:1.2,letterSpacing:-1,opacity:0,rotation:-90,borderRadius:0,objectFit:'contain',visible:false,locked:false,unknown:'removed'}],unknown:'removed'}});
  assert.equal(created.data.status,'draft');
  assert.equal(created.data.template_data.canvasWidth,1000);
  assert.equal(created.data.template_data.elements[0].visible,false);
  assert.equal(created.data.template_data.elements[0].unknown,undefined);
  const path='/v2/certificate-templates/'+created.data.id;
  await h.call('template:null-background-unchanged','PUT',path,{expectedVersion:1,backgroundImage:null});
  await h.call('template:partial-design','PUT',path,{expectedVersion:2,templateData:{canvasWidth:800}});
  await h.call('template:partial-design-copy-fails','POST',path+'/duplicate');
  await h.call('template:empty-design','PUT',path,{expectedVersion:3,templateData:{}});
  await h.call('template:empty-design-publish','POST',path+'/publish',{expectedVersion:4});
  await h.seed('UPDATE certificate_templates SET updated_at=null WHERE id=$1',[created.data.id]);
  await h.call('template:null-time-conflict','PUT',path,{expectedVersion:1});
  await h.seed("UPDATE certificate_templates SET version=2147483647,updated_at='2024-01-01' WHERE id=$1",[created.data.id]);
  await h.call('template:version-overflow-rollback','PUT',path,{expectedVersion:2147483647,name:'Must roll back'});
  await h.call('template:version-overflow-show','GET',path);
  await h.call('template:boundary-delete','DELETE',path);
}
