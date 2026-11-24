export const vert = `#version 300 es
in vec2 p; void main(){ gl_Position = vec4(p,0.,1.); }`;

// Everything is drawn in CSS pixels, origin top left.
export const frag = `#version 300 es
precision highp float;
uniform vec2 uRes; uniform float uDpr; uniform float uT;
uniform sampler2D uA; uniform sampler2D uB;   // picture leaving / arriving
uniform float uMix;                            // 0..1 change of picture
uniform float uObj;                            // how present the floating picture is
uniform vec4 uRect; uniform float uH;          // where it floats; the waterline
uniform float uCut;                            // 1 = has its own outline
uniform float uCutA;
uniform vec2 uStone; uniform float uStoneOn;
uniform vec3 uRip[6];
out vec4 o;

float hash(vec2 p){ return fract(sin(dot(p,vec2(12.9898,78.233)))*43758.5453); }
vec3 sat(vec3 c,float s){ float l=dot(c,vec3(.3,.59,.11)); return max(mix(vec3(l),c,s),0.); }

vec3 field(sampler2D t, vec2 uv){
  vec3 c=vec3(0.);
  for(int i=0;i<14;i++){
    float a=float(i)*2.39996, r=sqrt(float(i)+.5)*.072;
    c+=textureLod(t, clamp(uv+vec2(cos(a),sin(a))*r,.06,.94), 5.).rgb;
  }
  return c/14.;
}
vec3 air(vec2 px){
  vec2 u=px/uRes;
  float t=uT*.045;
  u+= .13*vec2(sin(u.y*2.6+t*2.1)+.5*sin(u.y*5.1-t*1.7), cos(u.x*2.2-t*1.6)+.5*sin(u.x*4.4+t*1.9));
  u=mix(vec2(.5),u,.72);
  vec3 c=mix(field(uA,u),field(uB,u),smoothstep(.1,.9,uMix));
  c=sat(c,1.55);
  float l=dot(c,vec3(.3,.59,.11));
  c*= .52/(.16+l*1.15);               // pull bright and dark covers to one exposure
  return c*.74+vec3(.012,.016,.021);
}
vec4 pict(vec2 px, float blur){
  float up=smoothstep(.42,1.,uMix), down=smoothstep(0.,.45,uMix);
  up=1.-pow(1.-up,3.);
  float bob=sin(uT*.85)*3.;
  vec4 r=uRect; r.y+=bob;
  vec2 q=px-r.xy;
  float cut=uMix<.45?uCutA:uCut;
  q.y-= uMix<.45 ? down*down*r.w*1.06 : (1.-up)*r.w*1.06;
  q.y-= cut*6.;
  vec2 uv=q/r.zw;
  if(uv.x<0.||uv.x>1.||uv.y<0.||uv.y>1.||px.y>uH+1.) return vec4(0.);
  vec4 c= uMix<.45 ? textureLod(uA,uv,blur) : textureLod(uB,uv,blur);
  vec2 d=abs(q-r.zw*.5)-(r.zw*.5-5.);
  float e=length(max(d,0.))-5.;
  c.a*= mix(smoothstep(.8,-.8,e),1.,cut);
  return c*vec4(vec3(1.),uObj);
}
vec3 sky(vec2 px, float blur){
  vec3 c=air(px);
  float y=px.y/uH;
  c*= mix(.50,1.06,smoothstep(-.1,1.,y));
  vec2 m=(px-(uRect.xy+uRect.zw*.5))/uRect.zw;
  c+= air(uRect.xy+uRect.zw*.5)*.55*exp(-dot(m,m)*1.5)*uObj;
  c+= air(vec2(px.x,uH))*.35*exp(-abs(px.y-uH)/46.);
  vec4 k=pict(px,blur);
  c=mix(c,k.rgb,k.a);
  if(uStoneOn>.001){
    vec2 s=(px-uStone); float r=length(s);
    c+= vec3(.96,.74,.30)*uStoneOn*(1.25*smoothstep(8.,5.,length(s*vec2(.92,1.2)))+.32*exp(-r/15.)+.12*exp(-r/60.));
  }
  return c;
}
void main(){
  vec2 px=vec2(gl_FragCoord.x, uRes.y*uDpr-gl_FragCoord.y)/uDpr;
  vec3 c;
  if(px.y<uH){ c=sky(px,0.); }
  else {
    float d=(px.y-uH);
    float dn=d/max(uRes.y-uH,1.);
    float z=1./(dn+.055);
    vec2 w=vec2((px.x-uRes.x*.5)/uRes.y*z*1.6, z);
    float t=uT;
    vec2 n=vec2(0.);
    n.x+= sin(w.x*3.1+w.y*1.3+t*.9)*.50+sin(w.x*7.3-w.y*2.1-t*1.25)*.26+sin(w.x*13.7+w.y*4.4+t*1.8)*.12;
    n.y+= sin(w.y*5.2-t*1.35+w.x*.8)*.60+sin(w.y*9.6-t*2.0-w.x*2.6)*.30+sin(w.y*17.-t*2.7+w.x*5.)*.14;
    for(int i=0;i<6;i++){
      float age=uT-uRip[i].z; if(age<0.||age>5.) continue;
      vec2 q=px-uRip[i].xy; q.y*=2.9;
      float r=length(q), f=age*150.;
      float ring=sin((r-f)*.085)*exp(-pow((r-f)/78.,2.))*exp(-age*.75)*smoothstep(0.,.1,age);
      n+= normalize(q+.001)*ring*5.;
    }
    float amp=mix(.8,15.,pow(dn,.75));
    vec2 rp=vec2(px.x+n.x*amp*.85, uH-d*1.02+n.y*amp*1.15);
    rp.y=min(rp.y,uH-.5);
    c=sky(rp,1.+dn*3.5);
    float fres=mix(.86,.42,smoothstep(0.,.9,dn));
    c=c*fres+air(vec2(px.x,uH*.4))*.10;
    c+= vec3(.96,.76,.34)*uStoneOn*exp(-abs(px.x+n.x*amp*1.2-uStone.x)/(7.+dn*26.))*exp(-dn*3.2)*(.30+.42*max(n.y,0.));
    c+= air(vec2(px.x,uH))*.9*pow(max(n.y*.62,0.),6.)*(1.-dn);
    c*= mix(1.,.66,dn);
    c+= air(vec2(px.x,uH))*.5*exp(-d/1.6);
  }
  c*= 1.-.17*pow(length((px/uRes-.5)*vec2(1.,.85))*1.35,2.4);
  c+= (hash(px+fract(uT))-.5)*.020;
  o=vec4(c,1.);
}`;
