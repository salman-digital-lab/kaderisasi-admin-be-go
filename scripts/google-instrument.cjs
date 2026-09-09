const {createRequire}=require('node:module');
const {resolve}=require('node:path');
const endpoint=new URL(process.env.GO_REWRITE_GOOGLE_KEYS_URL);
if(process.env.NODE_ENV!=='test'||!/^go_rewrite_[a-f0-9]{16}_baseline$/.test(process.env.DB_SCHEMA)||endpoint.protocol!=='http:'||endpoint.hostname!=='127.0.0.1')throw new Error('Google fixture transport requires an isolated test schema');
const legacyRequire=createRequire(resolve(__dirname,'../../kaderisasi-admin-be/package.json'));
const {OAuth2Client}=legacyRequire('google-auth-library');
const original=OAuth2Client.prototype.getFederatedSignonCertsAsync;
OAuth2Client.prototype.getFederatedSignonCertsAsync=function(){
  this.endpoints={...this.endpoints,oauth2FederatedSignonPemCertsUrl:new URL('/pem',endpoint),oauth2FederatedSignonJwkCertsUrl:endpoint};
  return original.call(this);
};
