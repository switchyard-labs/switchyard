package app

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"switchyard/internal/trestle"
)

const avatarMaxBytes = 2 << 20

func profileCollections() [][2]any {
	return [][2]any{
		{"user_profiles", []trestle.CollectionField{
			{Name:"username",Type:"text",Unique:true},{Name:"bio",Type:"text"},{Name:"location",Type:"text"},{Name:"website",Type:"text"},{Name:"social",Type:"json"},{Name:"avatar_file",Type:"text"},{Name:"updated_at",Type:"text"},
		}},
		{"org_profiles", []trestle.CollectionField{
			{Name:"org_id",Type:"text",Unique:true},{Name:"display_name",Type:"text"},{Name:"description",Type:"text"},{Name:"location",Type:"text"},{Name:"website",Type:"text"},{Name:"contact",Type:"text"},{Name:"avatar_file",Type:"text"},{Name:"visibility",Type:"text"},{Name:"updated_at",Type:"text"},
		}},
	}
}

func (a *App) userRecord(username string) map[string]any {
	it, err := a.Trestle.ListRecords("users", `username = "`+username+`"`); if err!=nil||len(it)==0{return nil}; return it[0]
}
func (a *App) userProfile(username string) map[string]any {
	p:=map[string]any{"username":username,"bio":"","location":"","website":"","social":map[string]any{},"avatar_url":"/api/avatars/user/"+username}
	if u:=a.userRecord(username);u!=nil { p["display_name"]=u["display_name"] }
	if it,err:=a.Trestle.ListRecords("user_profiles",`username = "`+username+`"`);err==nil&&len(it)>0{for k,v:=range it[0]{p[k]=v}}
	return p
}

func (a *App) handleGetUserProfile(w http.ResponseWriter,r *http.Request){u:=normalizeOwnerSlug(r.PathValue("username"));if a.userRecord(u)==nil{writeJSON(w,404,map[string]any{"error":"user_not_found"});return};writeJSON(w,200,a.userProfile(u))}
func (a *App) handleUpdateUserProfile(w http.ResponseWriter,r *http.Request){
	u:=a.currentUser(r);if u==""{writeJSON(w,401,map[string]any{"error":"unauthorized"});return}
	var in struct{DisplayName string `json:"display_name"`;Bio string `json:"bio"`;Location string `json:"location"`;Website string `json:"website"`;Social map[string]string `json:"social"`;PinnedRepos []string `json:"pinned_repos"`}
	if readJSON(r,&in)!=nil{writeJSON(w,400,map[string]any{"error":"bad_request"});return}
	if len(in.Bio)>500||len(in.Location)>120||len(in.Website)>300{writeJSON(w,400,map[string]any{"error":"profile_field_too_long"});return}
	if rid,ver,_,e:=a.Trestle.FindRecord("users",`username = "`+u+`"`);e==nil&&rid!=""&&strings.TrimSpace(in.DisplayName)!=""{_ = a.Trestle.PatchRecord("users",rid,ver,map[string]any{"display_name":strings.TrimSpace(in.DisplayName)})}
	vals:=map[string]any{"username":u,"bio":strings.TrimSpace(in.Bio),"location":strings.TrimSpace(in.Location),"website":strings.TrimSpace(in.Website),"social":in.Social,"pinned_repos":in.PinnedRepos,"updated_at":nowStr()}
	if rid,ver,old,e:=a.Trestle.FindRecord("user_profiles",`username = "`+u+`"`);e==nil&&rid!=""{if av,_:=old["avatar_file"].(string);av!=""{vals["avatar_file"]=av};_ = a.Trestle.PatchRecord("user_profiles",rid,ver,vals)}else{_,_,_ = a.Trestle.CreateRecord("user_profiles",vals,"profile-"+u)}
	writeJSON(w,200,a.userProfile(u))
}

func avatarExt(header []byte) (string,string,bool){ct:=http.DetectContentType(header);switch ct{case "image/png":return ".png",ct,true;case "image/jpeg":return ".jpg",ct,true;case "image/webp":return ".webp",ct,true;case "image/gif":return ".gif",ct,true};return "",ct,false}
func saveAvatarFile(dataDir,kind,id string,file multipart.File)(string,error){
	b,err:=io.ReadAll(io.LimitReader(file,avatarMaxBytes+1));if err!=nil{return "",err};if len(b)>avatarMaxBytes{return "",fmt.Errorf("avatar_too_large")};ext,_,ok:=avatarExt(b);if !ok{return "",fmt.Errorf("avatar_type_invalid")}
	dir:=filepath.Join(dataDir,"avatars",kind);if err:=os.MkdirAll(dir,0700);err!=nil{return "",err};name:=id+ext; if err:=os.WriteFile(filepath.Join(dir,name),b,0600);err!=nil{return "",err};return name,nil
}
func (a *App) handleUploadUserAvatar(w http.ResponseWriter,r *http.Request){u:=a.currentUser(r);if u==""{writeJSON(w,401,map[string]any{"error":"unauthorized"});return};if err:=r.ParseMultipartForm(avatarMaxBytes);err!=nil{writeJSON(w,400,map[string]any{"error":"bad_multipart"});return};f,_,err:=r.FormFile("avatar");if err!=nil{writeJSON(w,400,map[string]any{"error":"avatar_required"});return};defer f.Close();name,err:=saveAvatarFile(a.DataDir,"user",u,f);if err!=nil{writeJSON(w,400,map[string]any{"error":err.Error()});return};vals:=map[string]any{"username":u,"avatar_file":name,"updated_at":nowStr()};if rid,ver,_,e:=a.Trestle.FindRecord("user_profiles",`username = "`+u+`"`);e==nil&&rid!=""{_ = a.Trestle.PatchRecord("user_profiles",rid,ver,vals)}else{_,_,_=a.Trestle.CreateRecord("user_profiles",vals,"profile-avatar-"+u)};writeJSON(w,200,map[string]any{"avatar_url":"/api/avatars/user/"+u})}
func (a *App) handleDeleteUserAvatar(w http.ResponseWriter,r *http.Request){u:=a.currentUser(r);if u==""{writeJSON(w,401,map[string]any{"error":"unauthorized"});return};if rid,ver,v,e:=a.Trestle.FindRecord("user_profiles",`username = "`+u+`"`);e==nil&&rid!=""{if n,_:=v["avatar_file"].(string);n!=""{_ = os.Remove(filepath.Join(a.DataDir,"avatars","user",n))};_ = a.Trestle.PatchRecord("user_profiles",rid,ver,map[string]any{"avatar_file":"","updated_at":nowStr()})};writeJSON(w,200,map[string]any{"ok":true})}
func fallbackAvatar(label string) string {initial:="?";if s:=strings.TrimSpace(label);s!=""{initial=strings.ToUpper(string([]rune(s)[0]))};return `<svg xmlns="http://www.w3.org/2000/svg" width="256" height="256" viewBox="0 0 256 256"><rect width="256" height="256" rx="32" fill="#202226"/><circle cx="128" cy="128" r="104" fill="#292c31" stroke="#454a51" stroke-width="2"/><text x="128" y="153" text-anchor="middle" font-family="system-ui,sans-serif" font-size="88" font-weight="700" fill="#d9a45b">`+initial+`</text></svg>`}
func (a *App) handleAvatar(w http.ResponseWriter,r *http.Request){kind,id:=r.PathValue("kind"),r.PathValue("id");var name string;if kind=="user"{if p:=a.userProfile(id);p!=nil{name,_=p["avatar_file"].(string)}}else if kind=="org"{if it,e:=a.Trestle.ListRecords("org_profiles",`org_id = "`+id+`"`);e==nil&&len(it)>0{name,_=it[0]["avatar_file"].(string)}};if name!=""{p:=filepath.Join(a.DataDir,"avatars",kind,name);if b,e:=os.ReadFile(p);e==nil{w.Header().Set("Content-Type",http.DetectContentType(b));w.Header().Set("Cache-Control","private, max-age=300");_,_=w.Write(b);return}};w.Header().Set("Content-Type","image/svg+xml");_,_=w.Write([]byte(fallbackAvatar(id)))}

func (a *App) handleUserRepositories(w http.ResponseWriter,r *http.Request){
	u:=normalizeOwnerSlug(r.PathValue("username")); if a.userRecord(u)==nil{writeJSON(w,404,map[string]any{"error":"user_not_found"});return}
	items,err:=a.Trestle.ListRecords("repository_meta",`owner_type = "user"`);if err!=nil{writeJSON(w,502,map[string]any{"error":err.Error()});return}
	out:=[]map[string]any{};for _,it:=range items{if it["owner_id"]==u||it["owner_slug"]==u{out=append(out,it)}}
	writeJSON(w,200,map[string]any{"items":out})
}
func (a *App) handleUserActivity(w http.ResponseWriter,r *http.Request){
	u:=normalizeOwnerSlug(r.PathValue("username")); if a.userRecord(u)==nil{writeJSON(w,404,map[string]any{"error":"user_not_found"});return}
	out:=[]map[string]any{}
	if work,e:=a.Trestle.ListRecords("work",`owner = "`+u+`"`);e==nil{for _,x:=range work{out=append(out,map[string]any{"type":"work","title":x["title"],"at":x["updated_at"],"id":x["id"]})}}
	if refs,e:=a.Trestle.ListRecords("ref_updates","");e==nil{for _,x:=range refs{if strings.Contains(strOr(x["provenance"]),u){out=append(out,map[string]any{"type":"git","repo":x["repo"],"branch":x["branch"],"at":x["occurred_at"],"sha":x["new_sha"]})}}}
	if len(out)>40{out=out[len(out)-40:]};writeJSON(w,200,map[string]any{"items":out})
}
