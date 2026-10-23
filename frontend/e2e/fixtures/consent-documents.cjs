const fs=require('node:fs');
const path=require('node:path');
const crypto=require('node:crypto');
module.exports=['terms','privacy'].map(name=>{
 const bytes=fs.readFileSync(path.join(__dirname,'../../../internal/identity/consentdocs',name+'.json'));
 const document=JSON.parse(bytes);
 return {id:document.id,code:document.code,type:document.type,sha256:crypto.createHash('sha256').update(bytes).digest('hex'),published_at:'2026-10-23T00:00:00Z',synthetic:true,document};
});
