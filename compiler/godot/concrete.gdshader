shader_type spatial;
render_mode cull_disabled;
// One concrete, four maintenance histories. Added deposits have zero support
// outside actual CUE anchors; seed changes only map phase/anchored modulation.
uniform sampler2D concrete_albedo : source_color, filter_linear_mipmap, repeat_enable;
uniform sampler2D concrete_normal : hint_normal, filter_linear_mipmap, repeat_enable;
uniform sampler2D concrete_roughness : filter_linear_mipmap, repeat_enable;
uniform sampler2D repairs_atlas : source_color, filter_linear_mipmap;
uniform vec3 tint = vec3(0.5);
uniform vec3 stain_color = vec3(0.06,0.055,0.045);
uniform vec3 rust_color = vec3(0.19,0.075,0.03);
uniform vec3 stripe_color = vec3(0.20,0.28,0.32);
uniform vec3 resin_color = vec3(0.63,0.43,0.13);
uniform float resin_metallic = 0.7;
uniform float texture_scale = 0.25;
uniform float roughness_base = 0.85;
uniform float pore_contrast = 1.0;
uniform float normal_strength = 1.0;
uniform float water = 0.0;
uniform float rust = 0.0;
uniform float patches = 0.0;
uniform float soot = 0.0;
uniform float protocol_stripe = 0.0;
uniform float damage = 0.0;
uniform int tier = 1;
uniform float age = 0.7;
uniform float floor_m = 0.0;
uniform bool exposure_public = true;
uniform float seed_phase = 0.0;
uniform vec3 review_origin = vec3(0.0);
uniform int anchor_count = 0;
uniform vec3 anchor_position[64];
uniform vec4 anchor_meta[64];
uniform int patch_count = 0;
uniform vec3 patch_position[16];
uniform vec4 patch_size[16];
uniform vec4 patch_uv[16];
uniform int stripe_count = 0;
uniform vec4 stripe_a[16];
uniform vec4 stripe_b[16];
uniform int resin_count = 0;
uniform vec3 resin_a[16];
uniform vec3 resin_b[16];
uniform int route_count = 0;
uniform vec4 route_a[128];
uniform vec4 route_b[128];
uniform int caries_count = 0;
uniform vec3 caries_position[16];
uniform vec4 caries_meta[16];
uniform int ghost_count = 0;
uniform vec3 ghost_position[8];
uniform vec4 ghost_size[8];
varying vec3 wall_position;
varying vec3 wall_normal;
float segment_distance(vec3 p,vec3 a,vec3 b){vec3 d=b-a;float q=dot(d,d);float t=q>0.000001?clamp(dot(p-a,d)/q,0.0,1.0):0.0;return length(p-a-d*t);}
float window(float p,float lo,float hi,float feather){return smoothstep(lo-feather,lo,p)*(1.0-smoothstep(hi,hi+feather,p));}
vec2 wall_coordinates(vec3 p,vec3 n){return abs(n.x)>abs(n.z)?vec2(p.z,p.y):vec2(p.x,p.y);}
void vertex(){wall_position=(MODEL_MATRIX*vec4(VERTEX,1.0)).xyz;wall_normal=normalize(MODEL_NORMAL_MATRIX*NORMAL);}
void fragment(){
 vec3 wp=wall_position-review_origin;vec3 n=normalize(wall_normal);vec3 weights=pow(abs(n),vec3(6.0));weights/=max(0.0001,weights.x+weights.y+weights.z);
 vec3 phase=vec3(seed_phase*4.0,0.0,seed_phase*4.0);
 vec2 ux=(wp+phase).zy*texture_scale;vec2 uy=(wp+phase).xz*texture_scale;vec2 uz=(wp+phase).xy*texture_scale;
 vec3 base=texture(concrete_albedo,ux).rgb*weights.x+texture(concrete_albedo,uy).rgb*weights.y+texture(concrete_albedo,uz).rgb*weights.z;
 float source_rough=texture(concrete_roughness,ux).r*weights.x+texture(concrete_roughness,uy).r*weights.y+texture(concrete_roughness,uz).r*weights.z;
 float h=wp.y-floor_m;vec2 uvwall=wall_coordinates(wp,n);
 float wet=0.0;float corrosion=0.0;float soot_mask=0.0;float oil=0.0;float gate_marks=0.0;float near_threshold=0.0;
 for(int i=0;i<64;i++){if(i>=anchor_count){break;}vec3 a=anchor_position[i];vec4 m=anchor_meta[i];vec2 av=wall_coordinates(a,n);float dx=abs(uvwall.x-av.x);float down=a.y-wp.y;float up=wp.y-a.y;float width=max(0.04,m.z);float plane_support=1.0-smoothstep(0.2,0.65,abs(dot(wp-a,n)));float support=(1.0-smoothstep(width*0.5,width,dx))*plane_support;float variation=0.87+0.13*sin(dx*8.1+down*3.7+seed_phase*6.283);
  if(m.x<0.5){wet=max(wet,support*window(down,0.0,3.0,0.12)*m.y*variation);}
  else if(m.x<1.5){corrosion=max(corrosion,support*window(down,0.0,mix(0.3,0.8,age),0.06)*m.y*variation);oil=max(oil,support*window(h,0.0,0.6,0.1)*m.y);}
  else if(m.x<3.5){float cone_width=max(0.10,0.18+up*0.65);float cone=(1.0-smoothstep(cone_width*0.5,cone_width,dx))*window(up,0.0,0.9,0.08);soot_mask=max(soot_mask,cone*m.y*variation*plane_support);}
  else{float distance_xz=length(wp.xz-a.xz);near_threshold=max(near_threshold,1.0-smoothstep(1.5,2.0,distance_xz));gate_marks=max(gate_marks,support*window(h,0.5,1.0,0.1)*m.y);}
 }
 vec3 color=tint*mix(vec3(1.0),base*2.0,pore_contrast);float rough=clamp(roughness_base+(source_rough-0.5)*0.2,0.06,1.0);
 if(tier==0){float damp=wet*window(h,0.0,1.2,0.15)*0.9;color=mix(color,stain_color,clamp(damp+wet*water*0.45+oil*0.4+gate_marks*0.25,0.0,0.9));rough=mix(rough,0.35,damp);color=mix(color,rust_color,corrosion*rust*age);color=mix(color,vec3(0.61,0.60,0.56),wet*window(h,0.3,1.0,0.08)*0.15);}
 else if(tier==1){color=mix(color,stain_color,wet*water*0.25+oil*0.3+gate_marks*0.15);color=mix(color,rust_color,corrosion*rust*age*0.5);}
 if(tier<2){color=mix(color,stain_color,soot_mask*soot);}
 // Corner/door wear is supported by the actual gate source, never free noise.
 float chips=gate_marks*damage*window(h,0.0,1.0,0.1);
 if(tier==2){color=mix(color,base*0.48,chips);rough=mix(rough,0.55,chips);}
 if(tier==3){rough=mix(rough,0.30,clamp((base.r-0.55)*1.5,0.0,1.0));}
 // The hand band is supported by actual build-time routes/use queues/narrows.
 float touch=0.0;for(int i=0;i<128;i++){if(i>=route_count){break;}vec3 a=route_a[i].xyz;vec3 b=route_b[i].xyz;vec3 p=vec3(wp.x,a.y,wp.z);touch=max(touch,(1.0-smoothstep(0.3,1.0,segment_distance(p,a,b)))*route_a[i].w);}
 touch*=window(h,0.9,1.5,0.12)*(exposure_public?1.0:0.45);
 if(tier==0){color=mix(color,stain_color,touch*0.8);rough=mix(rough,0.5,touch);}
 else if(tier==1){color=mix(color,color*1.12,touch*0.4);rough=mix(rough,0.9,touch*0.4);}
 else if(tier==2){color=mix(color,color*0.92,touch*near_threshold*0.2);}
 // Repairs sample the existing atlas. Small brass number/date tags are separate
 // physical plates and localized world-text slots; no glyph is drawn here.
 for(int i=0;i<16;i++){if(i>=patch_count){break;}vec2 p=wall_coordinates(patch_position[i],n);vec2 local=(uvwall-p)/max(patch_size[i].xy,vec2(0.001))+vec2(0.5);if(local.x>=0.0&&local.x<=1.0&&local.y>=0.0&&local.y<=1.0){vec4 repair=texture(repairs_atlas,mix(patch_uv[i].xy,patch_uv[i].zw,local));repair.a*=(1.0-smoothstep(0.025,0.08,abs(dot(wp-patch_position[i],n))))*(1.0-step(0.5,abs(n.y)));if(tier<2){color=mix(color,repair.rgb,repair.a);rough=mix(rough,0.7,repair.a);}else if(tier==2){float generation=mod(float(i),2.0)<0.5?0.985:1.015;color=mix(color,tint*generation,repair.a);rough=mix(rough,0.45,repair.a);}else{rough=mix(rough,0.19,repair.a*0.3);}}}
 // Silver guide segments preserve present/painted-over/missing data. Dust at a
 // gap is visual evidence only: no robot or stealth behavior is implemented.
 for(int i=0;i<16;i++){if(i>=stripe_count){break;}vec3 a=stripe_a[i].xyz;vec3 b=stripe_b[i].xyz;float band=1.0-smoothstep(stripe_b[i].w*0.5,stripe_b[i].w*0.5+0.006,segment_distance(wp,a,b));float status=stripe_a[i].w;
  if(status<0.5){color=mix(color,stripe_color,band);rough=mix(rough,0.48,band);}
  else{color=mix(color,tint*(status<1.5?1.025:0.97),band);vec3 mid=(a+b)*0.5;vec3 low=vec3(mid.x,floor_m+0.18,mid.z);float dust=(1.0-smoothstep(0.2,0.65,length(wp-low)))*0.16;color=mix(color,stain_color,dust);}
 }
 float resin=0.0;for(int i=0;i<16;i++){if(i>=resin_count){break;}resin=max(resin,1.0-smoothstep(0.007,0.013,segment_distance(wp,resin_a[i],resin_b[i])));}if(tier==3){color=mix(color,resin_color,resin);rough=mix(rough,0.19,resin);}
 // Historical machine rectangles, real anchor rows and wordless cult tallies.
 // Gold deliberately receives none, enforced by the CUE validator.
 for(int i=0;i<8;i++){if(i>=ghost_count){break;}vec2 p=wall_coordinates(ghost_position[i],n);vec2 d=uvwall-p;vec2 size=ghost_size[i].xy;float inside=window(d.x,-size.x*0.5,size.x*0.5,0.025)*window(d.y,-size.y*0.5,size.y*0.5,0.025)*(1.0-smoothstep(0.025,0.08,abs(dot(wp-ghost_position[i],n))))*(1.0-step(0.5,abs(n.y)));color=mix(color,color*(tier==2?1.025:1.24),inside*(tier==2?0.4:0.65));
  vec2 counts=max(ghost_size[i].zw,vec2(1.0));vec2 grid=size/max(counts-vec2(1.0),vec2(1.0));vec2 q=abs(mod(d+size*0.5+grid*0.5,grid)-grid*0.5);float hole=(1.0-smoothstep(0.018,0.028,length(q)))*inside; color=mix(color,vec3(0.035),hole*(tier==2?0.15:0.9));
  if(tier<2){vec2 tally=d-vec2(size.x*0.35,-size.y*0.3);float bars=window(tally.y,-0.1,0.1,0.005)*(1.0-smoothstep(0.008,0.014,abs(mod(tally.x+0.02,0.05)-0.025)))*window(tally.x,0.0,0.2,0.005);float slash=(1.0-smoothstep(0.008,0.014,abs(tally.y-tally.x+0.10)))*window(tally.x,0.0,0.2,0.005);color=mix(color,vec3(0.65),max(bars,slash)*inside);}
 }
 // Hidden hints affect real wall fragments at the declared physical source;
 // matching thin CUE film geometry also exists behind actual covers/jambs.
 for(int i=0;i<16;i++){if(i>=caries_count){break;}vec3 a=caries_position[i];vec4 m=caries_meta[i];float mask=(1.0-smoothstep(m.y*0.45,m.y,length(wp-a)))*m.x;vec3 deposit=m.z<0.5?vec3(0.018,0.021,0.017):(m.z<1.5?vec3(0.14,0.13,0.11):(m.z<2.5?vec3(0.08,0.16,0.10):vec3(0.006)));color=mix(color,deposit,mask);rough=mix(rough,m.z<0.5?0.35:0.9,mask);}
 ALBEDO=clamp(color,vec3(0.0),vec3(1.0));METALLIC=tier==3?resin*resin_metallic:0.0;ROUGHNESS=rough;
 vec3 t;vec3 b;vec2 normal_uv;
 if(abs(n.y)>abs(n.x)&&abs(n.y)>abs(n.z)){t=vec3(1,0,0);b=vec3(0,0,-sign(n.y));normal_uv=vec2(wp.x,-wp.z*sign(n.y))*texture_scale;}
 else if(abs(n.x)>abs(n.z)){t=vec3(0,0,-sign(n.x));b=vec3(0,1,0);normal_uv=vec2(-wp.z*sign(n.x),wp.y)*texture_scale;}
 else{t=vec3(sign(n.z),0,0);b=vec3(0,1,0);normal_uv=vec2(wp.x*sign(n.z),wp.y)*texture_scale;}
 vec3 detail=texture(concrete_normal,normal_uv+vec2(seed_phase)).xyz*2.0-1.0;detail.xy*=normal_strength;
 NORMAL=normalize((VIEW_MATRIX*vec4(normalize(t*detail.x+b*detail.y+n*max(0.1,detail.z)),0.0)).xyz);
}
