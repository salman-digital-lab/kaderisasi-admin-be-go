import assert from 'node:assert/strict';
export async function achievementCases(h){
  await h.call('counseling:member','POST','/v2/members',{name:'Achievement member',gender:'F',email:'achievement@example.test'});
  await h.seed("INSERT INTO ruang_curhats(user_id,counselor_id,problem_description,created_at,updated_at) VALUES(1,1,'Synthetic counseling','2024-01-01','2024-01-01')");
  await h.call('counseling:list','GET','/v2/ruang-curhat');
  await h.call('counseling:filtered','GET','/v2/ruang-curhat?name=Achievement&gender=F&status=0&admin_display_name=Super');
  await h.call('counseling:show','GET','/v2/ruang-curhat/1');
  await h.call('counseling:update','PUT','/v2/ruang-curhat/1',{counselor_id:2,status:1,additional_notes:'Synthetic notes'});
  await h.call('counseling:invalid','PUT','/v2/ruang-curhat/1',{status:'wrong'});
  await h.call('counseling:missing','GET','/v2/ruang-curhat/999999');
  await h.seed("INSERT INTO achievements(user_id,name,type,score,achievement_date,created_at,updated_at) VALUES(1,'Synthetic award',2,10,'2026-02-28','2024-01-01','2024-01-01'),(1,'Other award',0,5,'2025-12-31','2024-01-02','2024-01-02')");
  await h.call('achievement:list','GET','/v2/achievements');
  await h.call('achievement:filtered','GET','/v2/achievements?name=Achievement&type=2&status=0&email=achievement@example.test&sort_by=achievement_date&sort_order=asc');
  await h.call('achievement:show','GET','/v2/achievements/1');
  await h.call('achievement:update','PUT','/v2/achievements/1',{description:'Synthetic description',score:15});
  await h.call('achievement:invalid','PUT','/v2/achievements/1',{score:-1,type:1.5});
  await h.call('achievement:missing-status','PUT','/v2/achievements/1/approve-reject',{score:20});
  await h.call('achievement:approve','PUT','/v2/achievements/1/approve-reject',{status:1,score:20});
  await h.call('achievement:show-approved','GET','/v2/achievements/1');
  await h.call('achievement:repeat-approve','PUT','/v2/achievements/1/approve-reject',{status:1,score:20});
  await h.call('achievement:reject','PUT','/v2/achievements/1/approve-reject',{status:2,remark:'Synthetic rejection'});
  await h.call('achievement:approve-update','PUT','/v2/achievements/2',{status:1});
  await h.call('achievement:list-reviewed','GET','/v2/achievements');
  await h.call('leaderboard:monthly','GET','/v2/leaderboards/monthly');
  await h.call('leaderboard:monthly-filter','GET','/v2/leaderboards/monthly?month=2&year=2026&name=Achievement&email=achievement');
  await h.call('leaderboard:year','GET','/v2/leaderboards/monthly?year=2026');
  await h.call('leaderboard:empty-month','GET','/v2/leaderboards/monthly?month=1&year=2026');
  await h.call('leaderboard:lifetime','GET','/v2/leaderboards/lifetime?name=Achievement');
  await h.call('achievement:export','GET','/v2/achievements/export');
  await h.call('achievement:missing','GET','/v2/achievements/999999');
  await h.call('achievement:missing-update','PUT','/v2/achievements/999999',{name:'Missing'});
  await h.call('achievement:missing-review','PUT','/v2/achievements/999999/approve-reject',{status:1});
  await counselingBoundaryCases(h);
  await achievementBoundaryCases(h);
}

async function counselingBoundaryCases(h){
  await h.seed("UPDATE ruang_curhats SET updated_at='2026-01-01T00:00:00Z' WHERE id=1");
  await h.call('counseling:empty-update','PUT','/v2/ruang-curhat/1',{});
  await h.call('counseling:null-fields','PUT','/v2/ruang-curhat/1',{counselor_id:null,status:null,additional_notes:''});
  for(const id of ['bad','1.5','2147483648','%31','01']){
    await h.call('counseling:show-id:'+id,'GET','/v2/ruang-curhat/'+id);
    await h.call('counseling:update-id:'+id,'PUT','/v2/ruang-curhat/'+id,{});
  }
  for(const value of [0.5,2147483648,999999]){
    await h.call('counseling:counselor-boundary:'+value,'PUT','/v2/ruang-curhat/1',{counselor_id:value});
    await h.call('counseling:status-boundary:'+value,'PUT','/v2/ruang-curhat/1',{status:value});
  }
  for(const query of ['status=bad','status=0.5','status=2147483648','per_page=-1','name=Achievement&per_page=-1','gender=F&per_page=-1','name=Achievement&gender=F&per_page=-1','admin_display_name=Requester&per_page=-1','status=999999&name=Achievement&gender=F&admin_display_name=Requester&per_page=-1'])await h.call('counseling:query:'+query,'GET','/v2/ruang-curhat?'+query);
  await h.seed('UPDATE ruang_curhats SET user_id=null,counselor_id=null WHERE id=1');
  await h.call('counseling:null-relations','GET','/v2/ruang-curhat/1');
  await h.call('counseling:null-relation-list','GET','/v2/ruang-curhat');
}

async function achievementBoundaryCases(h){
  for(const id of ['bad','1.5','2147483648','%31','01'])for(const [method,suffix,body] of [['GET',''],['PUT','',{}],['PUT','/approve-reject',{status:1}]])await h.call(`achievement:id:${id}:${method}:${suffix}`,method,`/v2/achievements/${id}${suffix}`,body);
  await h.seed("UPDATE achievements SET status=1,updated_at='2026-01-01T00:00:00Z' WHERE id=1");
  const same=await h.call('achievement:unchanged','PUT','/v2/achievements/1',{status:1,score:20});
  assert.equal(new Date(same.data.updated_at).toISOString(),'2026-01-01T00:00:00.000Z');
  const reviewed=await h.call('achievement:repeat-review-touches-time','PUT','/v2/achievements/1/approve-reject',{status:1});
  assert.notEqual(new Date(reviewed.data.updated_at).toISOString(),'2026-01-01T00:00:00.000Z');
  for(const field of ['status','type','score'])for(const action of ['', '/approve-reject'])await h.call('achievement:wide:'+field+action,'PUT','/v2/achievements/1'+action,{status:1,[field]:2147483648});
  for(const query of ['status=bad','status=','type=2147483648','per_page=-1','name=Achievement&per_page=-1','email=achievement@example.test&per_page=-1','status=1&email=achievement@example.test&name=Achievement&type=2&per_page=-1'])await h.call('achievement:query:'+query,'GET','/v2/achievements?'+query);
  const created=await h.call('achievement:boundary-member','POST','/v2/members',{name:'Score boundary',email:'score-boundary@example.test'});
  const userID=created.data.user.id;
  await h.seed("INSERT INTO monthly_leaderboards(user_id,month,score,score_academic,score_competition,score_organizational,created_at,updated_at) VALUES($1,'2026-04-01',null,null,null,null,'2024-01-01','2024-01-01')",[userID]);
  await h.seed("INSERT INTO lifetime_leaderboards(user_id,score,score_academic,score_competition,score_organizational,created_at,updated_at) VALUES($1,null,null,null,null,'2024-01-01','2024-01-01')",[userID]);
  let lastID;
  for(const kind of [0,1,2,9]){
    const [row]=await h.seed("INSERT INTO achievements(user_id,name,type,score,achievement_date,created_at,updated_at) VALUES($1,'Boundary score',$2,5,'2026-04-15','2024-02-01'::timestamptz+$2::integer*interval '1 day','2024-01-01') RETURNING id",[userID,kind]);
    lastID=row.id;
    await h.call('achievement:category:'+kind,'PUT',`/v2/achievements/${row.id}/approve-reject`,{status:1});
  }
  const scores=await h.call('leaderboard:nullable-category-values','GET','/v2/leaderboards/lifetime?email=score-boundary');
  assert.equal(scores.data.data[0].score,20);
  assert.equal(scores.data.data[0].score_academic,5);
  assert.equal(scores.data.data[0].score_competition,5);
  assert.equal(scores.data.data[0].score_organizational,5);
  await h.seed('UPDATE lifetime_leaderboards SET score=2147483647 WHERE user_id=$1',[userID]);
  await h.call('achievement:overflow-rolls-back-both-boards','PUT',`/v2/achievements/${lastID}/approve-reject`,{status:1});
  await h.seed('UPDATE achievements SET achievement_date=null WHERE id=$1',[lastID]);
  await h.call('achievement:null-date-rolls-back','PUT',`/v2/achievements/${lastID}/approve-reject`,{status:1});
  await h.seed('UPDATE achievements SET name=null,score=null,type=null,status=null,approver_id=null,user_id=null WHERE id=$1',[lastID]);
  await h.call('achievement:legacy-null-read','GET',`/v2/achievements/${lastID}`);
  await h.call('achievement:legacy-null-export','GET','/v2/achievements/export');
  await h.seed("INSERT INTO monthly_leaderboards(user_id,month,score,created_at,updated_at) VALUES(null,null,3,'2024-01-01','2024-01-01')");
  for(const query of ['year=0x','year=2026&month=0x','year=%EF%BB%BF2026&month=2','year=2026x&month=2x','year=0x7ea&month=0x2','year=2026&month=13','year=2026&month=0','year=bad&month=2','year=2026&month=bad','year=2147483648&month=1','year=0&month=1','year=-1&month=1','year=10000&month=1','year=2026&per_page=-1','year=2026&month=2&email=achievement&name=Achievement&per_page=-1','year=bad'])await h.call('leaderboard:month-boundary:'+query,'GET','/v2/leaderboards/monthly?'+query);
  for(const query of ['per_page=-1','name=Achievement&per_page=-1','email=achievement&name=Achievement&per_page=-1'])await h.call('leaderboard:lifetime-boundary:'+query,'GET','/v2/leaderboards/lifetime?'+query);
}
