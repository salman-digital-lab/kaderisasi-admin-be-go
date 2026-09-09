import {legacyRequire} from './fixture-db.mjs';

export async function imageCases(h){
  await h.call('images:activity','POST','/v2/activities',{name:'Image edge fixture'});
  await h.call('images:club','POST','/v2/clubs',{name:'Image edge fixture'});
  await h.call('images:template','POST','/v2/certificate-templates',{name:'Image edge fixture'});
  const sharp=legacyRequire('sharp');
  const image=sharp({create:{width:120,height:80,channels:3,background:'#b43c78'}});
  const oriented=await image.clone().withMetadata({orientation:6}).jpeg().toBuffer();
  const gif=await image.clone().gif().toBuffer();
  const excessivePixels=await sharp({create:{width:6500,height:6500,channels:3,background:'#000000'}}).png().toBuffer();
  const paths=[
    ['/v2/activities/1/images',{},1,r=>r.data.image],
    ['/v2/clubs/1/logo',{},2,r=>r.data.logo],
    ['/v2/clubs/1/media/image',{media_type:'image'},5,r=>r.data.media.items[0].media_url],
    ['/v2/certificate-templates/1/background',{},5,r=>r.data.asset_key],
    ['/v2/certificate-templates/1/assets',{},5,r=>r.data.asset_key],
  ];
  for(const [path,fields,limit,key] of paths){
    const uploaded=await h.upload(`image:orientation:${path}`,path,fields,oriented);
    await h.inspectObject(key(uploaded),{width:80,height:120});
    await h.upload(`image:unsupported-gif:${path}`,path,fields,gif);
    await h.upload(`image:truncated-jpeg:${path}`,path,fields,oriented.subarray(0,80));
    await h.upload(`image:pixel-limit:${path}`,path,fields,excessivePixels);
    await h.upload(`image:file-limit:${path}`,path,fields,Buffer.concat([oriented,Buffer.alloc((limit<<20)+1)]));
    await h.upload(`image:multipart-limit:${path}`,path,fields,Buffer.alloc(7<<20,65));
  }
}
