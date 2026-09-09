import {readFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {root} from './env.mjs';
import {fixturePassword} from './fixture-db.mjs';

// These requests complement the module workflows with absent path resources
// and invalid bodies. All identifiers refer to this run's isolated schema.
export async function routeEdgeCases(h,routes){
  await h.call('edge:province','POST','/v2/provinces',{name:'Edge province'});
  await h.call('edge:city','POST','/v2/cities',{name:'Edge city',province_id:1});
  await h.call('edge:university','POST','/v2/universities',{name:'Edge university',provinceId:1});
  await h.call('edge:member','POST','/v2/members',{name:'Edge member',email:'edge@example.test'});
  await h.call('edge:activity','POST','/v2/activities',{name:'Edge activity'});
  await h.call('edge:club','POST','/v2/clubs',{name:'Edge club'});
  await h.call('edge:form','POST','/v2/custom-forms',{formName:'Edge form'});
  await h.call('edge:template','POST','/v2/certificate-templates',{name:'Edge template'});
  await h.call('edge:registration','POST','/v2/activities/1/registrations',{user_id:1,questionnaire_answer:{}});
  await h.call('edge:club-registration','POST','/v2/clubs/1/registrations',{member_id:1});
  await h.call('edge:club-approval','PUT','/v2/club-registrations/1',{status:'APPROVED'});
  await h.call('edge:role','POST','/v2/clubs/1/member-roles',{club_registration_id:1,role_name:'Coordinator'});
  await h.call('edge:ticket','POST','/v2/access-requests',{role_code:'club_manager',reason:'Synthetic edge request'},'requester');
  await h.seed("INSERT INTO ruang_curhats(user_id,counselor_id,problem_description,created_at,updated_at) VALUES(1,1,'Edge fixture','2024-01-01','2024-01-01')");
  await h.seed("INSERT INTO achievements(user_id,name,type,score,achievement_date,created_at,updated_at) VALUES(1,'Edge award',2,10,'2026-02-28','2024-01-01','2024-01-01')");

  const bodies={
    adminusers_controller:{update:{displayName:'Missing'},editPassword:{password:fixturePassword}},
    access_requests_controller:{cancel:{},approve:{},reject:{rejection_reason:'Synthetic rejection'}},
    universities_controller:{update:{name:'Missing',provinceId:1}},
    provinces_controller:{update:{name:'Missing'}},cities_controller:{update:{name:'Missing',province_id:1}},
    profiles_controller:{update:{name:'Missing'},updateRegionalAssignment:{alumni_regional_assignment:[]}},
    auth_controller:{updateMember:{email:'missing@example.test',password:fixturePassword}},
    members_controller:{generateAccount:{email:'missing@example.test',password:fixturePassword}},
    activities_controller:{update:{name:'Missing'},deleteImage:{image:'missing.webp'},reorderImages:{images:[]}},
    activity_registrations_controller:{store:{user_id:1,questionnaire_answer:{}},updateStatusBulk:{current_status:'TERDAFTAR',new_status:'DITERIMA'},updateStatusByListOfEmail:{emails:['edge@example.test'],status:'DITERIMA'}},
    ruang_curhats_controller:{update:{status:1}},leaderboards_controller:{update:{name:'Missing'},approveReject:{status:1}},
    clubs_controller:{update:{name:'Missing'},addYoutubeMedia:{media_url:'https://youtu.be/abc_DEF-123',media_type:'video',video_source:'youtube'},deleteMedia:{media_url:'missing.webp'},updateRegistrationInfo:{registration_info:'Missing'}},
    club_registrations_controller:{store:{member_id:1},update:{status:'APPROVED'}},
    club_member_roles_controller:{store:{club_registration_id:1,role_name:'Coordinator'},update:{role_name:'Coordinator'}},
    custom_forms_controller:{update:{formName:'Missing'},attachToClub:{clubId:1},attachToActivity:{activityId:1},detachFromClub:{},detachFromActivity:{},toggleActive:{}},
    certificate_templates_controller:{update:{expectedVersion:1,name:'Missing'},publish:{expectedVersion:1},archive:{expectedVersion:1},duplicate:{}},
    certificates_controller:{revoke:{reason:'Synthetic missing certificate'}},
  };
  const uploads=new Set(['uploadImage','uploadLogo','uploadImageMedia','uploadBackground','uploadAsset']);
  for(const route of routes.filter(route=>route.path.includes(':'))){
    const path=route.path.replace(/:code/g,'MISSING').replace(/:[^/]+/g,'999999');
    const name=`edge:missing-resource:${route.method}:${route.path}`;
    if(uploads.has(route.action))await h.upload(name,path,route.action==='uploadImageMedia'?{media_type:'image'}:{});
    else await h.call(name,route.method,path,bodies[route.controller]?.[route.action]);
  }

  const schemas=JSON.parse(readFileSync(resolve(root,'internal/validation/schemas.json'),'utf8'));
  const invalid={};
  for(const schema of Object.values(schemas))for(const [field,rule] of Object.entries(schema.args?.[0]??{}))invalid[field]=['object','record'].includes(rule.kind)?42:{};
  // These commands consume no body. Authentication, token and origin failures
  // are covered by auth/authorization contracts; malformed paths are separate.
  const bodyless=new Set(['migrate','refresh','logout','cancel','approve','duplicate','destroy','detachFromClub','detachFromActivity','toggleActive']);
  for(const route of routes.filter(route=>['POST','PUT','PATCH'].includes(route.method)&&!bodyless.has(route.action))){
    const path=route.path.replace(/:[^/]+/g,'1');
    await h.call(`edge:invalid-body:${route.method}:${route.path}`,route.method,path,invalid);
  }
}
