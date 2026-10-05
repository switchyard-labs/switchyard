(function(){"use strict";
 const kind=location.pathname.replace(/\/$/,"");
 if(!["/verify-email","/reset-password","/forgot-password"].includes(kind))return;
 let token=new URLSearchParams(location.hash.slice(1)).get("token")||new URLSearchParams(location.search).get("token")||"";
 history.replaceState(null,"",kind);
 window.addEventListener("hashchange",()=>{token=new URLSearchParams(location.hash.slice(1)).get("token")||"";history.replaceState(null,"",kind);const form=document.getElementById("recovery-form"),status=document.getElementById("recovery-status"),submit=document.getElementById("recovery-submit");if(form){form.hidden=false;status.textContent="";submit.disabled=!token;}});
 document.addEventListener("DOMContentLoaded",()=>{
  const $=id=>document.getElementById(id),form=$("recovery-form"),status=$("recovery-status"),submit=$("recovery-submit");if(!form)return;
  const reset=kind==="/reset-password",verify=kind==="/verify-email";
  $("recovery-title").textContent=verify?"Verify your email":reset?"Reset your password":"Forgot password?";
  $("recovery-copy").textContent=verify?"Confirm that this email belongs to you.":reset?"Choose a new password. Existing sessions will be signed out.":"Enter your account email to request a recovery link.";
  for(const name of ["email","password","confirm"]){const active=name==="email"?!verify&&!reset:reset;$("recovery-"+name+"-label").hidden=!active;const input=$("recovery-"+name+"-label").querySelector("input");input.disabled=!active;input.required=active;}
  submit.textContent=verify?"Verify email":reset?"Update password":"Send reset link";
  if((verify||reset)&&!token){status.textContent="This link is invalid or expired. Request another link.";submit.disabled=true;$("recovery-next").href=reset?"/forgot-password":"/settings.html";$("recovery-next").textContent=reset?"Request another reset link":"Open account settings";}
  form.addEventListener("submit",async event=>{event.preventDefault();if(submit.disabled)return;status.textContent="";
   if(reset&&form.new_password.value!==form.confirm_password.value){status.textContent="Passwords do not match.";form.confirm_password.focus();return;}
   submit.disabled=true;
   const body=verify?{token}:reset?{token,new_password:form.new_password.value,confirm_password:form.confirm_password.value}:{email:form.email.value};
   const path=verify?"/api/auth/verify-email":reset?"/api/auth/password-reset/complete":"/api/auth/password-reset/request";
   try{const response=await fetch(path,{method:"POST",credentials:"same-origin",headers:{"Content-Type":"application/json"},body:JSON.stringify(body)});const data=await response.json();if(!response.ok)throw new Error(data.error||"unavailable");
    token="";form.reset();form.hidden=true;status.textContent=verify?"Email verified.":reset?"Password updated. Sign in with your new password.":data.message;
    $("recovery-next").textContent=verify?"Continue to Switchyard":"Sign in";$("recovery-next").href=verify?"/":"/signin";
   }catch(error){status.textContent=error.message==="account_token_invalid"?"This link is invalid or expired. Request another link.":error.message==="password_confirmation_or_policy"?"Use matching passwords of 8–72 bytes.":error.message==="rate_limited"?"Too many attempts. Please try again later.":"This request could not complete. Please try again.";submit.disabled=false;if(verify||reset){$("recovery-next").href=reset?"/forgot-password":"/settings.html";$("recovery-next").textContent="Request another link";}}
  });
 });
})();
