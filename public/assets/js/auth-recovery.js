(function(){"use strict";
 const kind=location.pathname.replace(/\/$/,"");
 if(!["/verify-email","/reset-password","/forgot-password"].includes(kind))return;
 let token=new URLSearchParams(location.hash.slice(1)).get("token")||new URLSearchParams(location.search).get("token")||"";
 history.replaceState(null,"",kind);
 let refreshLink=()=>{};
 window.addEventListener("hashchange",()=>{token=new URLSearchParams(location.hash.slice(1)).get("token")||"";history.replaceState(null,"",kind);refreshLink();});
 document.addEventListener("DOMContentLoaded",()=>{
  const $=id=>document.getElementById(id),form=$("recovery-form"),status=$("recovery-status"),submit=$("recovery-submit"),copy=$("recovery-copy"),next=$("recovery-next");if(!form)return;
  const reset=kind==="/reset-password",verify=kind==="/verify-email";
  $("recovery-title").textContent=verify?"Verify your email":reset?"Reset your password":"Forgot password?";
  const instructions=verify?"Confirm that this email belongs to you.":reset?"Choose a new password. Existing sessions will be signed out.":"Enter your account email to request a recovery link.";
  copy.textContent=instructions;
  for(const name of ["email","password","confirm"]){const active=name==="email"?!verify&&!reset:reset;$("recovery-"+name+"-label").hidden=!active;const input=$("recovery-"+name+"-label").querySelector("input");input.disabled=!active;input.required=active;}
  submit.textContent=verify?"Verify email":reset?"Update password":"Send reset link";
  function invalid(){form.hidden=true;copy.hidden=true;status.textContent="This link is invalid or expired. Request another link.";submit.disabled=true;next.href=reset?"/forgot-password":"/settings.html#security";next.textContent=reset?"Request another reset link":"Open account settings";}
  refreshLink=async()=>{
   if(!verify&&!reset){form.hidden=false;return;}
   const candidate=token;form.hidden=true;copy.hidden=true;submit.disabled=true;
   if(!candidate){invalid();return;}
   status.textContent="Checking this link…";
   try{
    const response=await fetch('/api/auth/account-token/validate?purpose='+(reset?'password_reset':'email_verification'),{method:'POST',credentials:'same-origin',headers:{'Content-Type':'application/json'},body:JSON.stringify({token:candidate})});
    if(candidate!==token)return;
    if(response.status===400){invalid();return;}
    if(!response.ok)throw Error();
    status.textContent="";form.hidden=false;copy.hidden=false;submit.disabled=false;
   }catch{if(candidate===token){status.textContent="This link could not be checked. Reload the email link to try again.";next.href=reset?'/forgot-password':'/settings.html#security';next.textContent='Request another link';}}
  };
  refreshLink();
  form.addEventListener("submit",async event=>{event.preventDefault();if(submit.disabled)return;status.textContent="";
   if(reset&&form.new_password.value!==form.confirm_password.value){status.textContent="Passwords do not match.";form.confirm_password.focus();return;}
   submit.disabled=true;
   const body=verify?{token}:reset?{token,new_password:form.new_password.value,confirm_password:form.confirm_password.value}:{email:form.email.value};
   const path=verify?"/api/auth/verify-email":reset?"/api/auth/password-reset/complete":"/api/auth/password-reset/request";
   try{const response=await fetch(path,{method:"POST",credentials:"same-origin",headers:{"Content-Type":"application/json"},body:JSON.stringify(body)});const data=await response.json();if(!response.ok)throw new Error(data.error||"unavailable");
    token="";form.reset();form.hidden=true;copy.hidden=true;status.textContent=verify?"Email verified.":reset?"Password updated. Sign in with your new password.":data.message;
    next.textContent=verify?"Continue to Switchyard":"Sign in";next.href=verify?"/":"/signin";
   }catch(error){if(error.message==="account_token_invalid"){invalid();return;}status.textContent=error.message==="password_confirmation_or_policy"?"Use matching passwords of 8–72 bytes.":error.message==="rate_limited"?"Too many attempts. Please try again later.":"This request could not complete. Please try again.";submit.disabled=false;if(verify||reset){next.href=reset?"/forgot-password":"/settings.html#security";next.textContent="Request another link";}}
  });
 });
})();
