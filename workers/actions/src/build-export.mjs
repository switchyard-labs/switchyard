// Fixed trusted uploader, executed after the approved build commands. Output
// paths and short-lived grants arrive through env, never interpolated shell.
export const buildExportScript = String.raw`
import fs from "node:fs";
import path from "node:path";
import {createHash} from "node:crypto";
import {Readable} from "node:stream";
const outputs=JSON.parse(process.env.SWITCHYARD_BUILD_OUTPUTS);
delete process.env.SWITCHYARD_BUILD_OUTPUTS;
const root=fs.realpathSync("/tmp/ci-source");
try {
 for(const output of outputs){
  const file=path.join(root,output.path),fd=fs.openSync(file,fs.constants.O_RDONLY|fs.constants.O_NOFOLLOW);
  try{
   const real=fs.realpathSync("/proc/self/fd/"+fd),stat=fs.fstatSync(fd);
   if(!real.startsWith(root+path.sep)||!stat.isFile()||stat.size<1||stat.size>output.limit)throw new Error("invalid_output");
   const hash=createHash("sha256");for await(const chunk of fs.createReadStream(file,{fd,autoClose:false,start:0}))hash.update(chunk);
   const response=await fetch(output.url,{method:"PUT",redirect:"error",headers:{Authorization:"Bearer "+output.token,"Content-Length":String(stat.size),"X-Checksum-SHA256":hash.digest("hex")},body:Readable.toWeb(fs.createReadStream(file,{fd,autoClose:false,start:0})),duplex:"half",signal:AbortSignal.timeout(60000)});
   if(!response.ok)throw new Error("upload_failed");await response.body?.cancel();
  }finally{fs.closeSync(fd);}
 }
 console.log("Approved release outputs uploaded");
}catch{console.error("Release output export failed");process.exitCode=1;}
`;
export const buildExportCommand = `node --input-type=module -e '${buildExportScript}'`;
