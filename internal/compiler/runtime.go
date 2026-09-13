package compiler

// runtimeC is deliberately kept separate from the frontend so it can grow into
// a versioned Ruby runtime without obscuring parsing and lowering code.
const runtimeC = `#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <ctype.h>

#if defined(__GNUC__) || defined(__TINYC__)
#define RUBYC_UNUSED __attribute__((unused))
#else
#define RUBYC_UNUSED
#endif
typedef struct { int kind; long n; char *s; } Value;
static Value RUBYC_UNUSED num(long n){return (Value){0,n,0};}
static Value str(char*s){return (Value){1,0,s};}
static Value nil(void){return (Value){2,0,0};}
static char *dup(const char*s){char*r=malloc(strlen(s)+1);strcpy(r,s);return r;}
static int RUBYC_UNUSED truth(Value v){return v.kind!=2 && (v.kind!=0 || v.n!=0);}
static char *text(Value v){char b[64];if(v.kind==1)return dup(v.s);if(v.kind==2)return dup("");snprintf(b,sizeof b,"%ld",v.n);return dup(b);}
static void out(Value v,int nl){if(v.kind==1)fputs(v.s,stdout);else if(v.kind==2)fputs("nil",stdout);else printf("%ld",v.n);if(nl)putchar('\n');}
static Value RUBYC_UNUSED add(Value a,Value b){if(!a.kind&&!b.kind)return num(a.n+b.n);char *as=text(a),*bs=text(b),*r=malloc(strlen(as)+strlen(bs)+1);strcpy(r,as);strcat(r,bs);return str(r);}
static int RUBYC_UNUSED eq(Value a,Value b){if(a.kind!=b.kind)return 0;return a.kind==1?strcmp(a.s,b.s)==0:a.kind==2||a.n==b.n;}
static Value RUBYC_UNUSED length(Value v){return num((long)strlen(text(v)));}
static Value RUBYC_UNUSED upper(Value v){char*s=text(v);for(char*p=s;*p;p++)*p=(char)toupper((unsigned char)*p);return str(s);}
static Value RUBYC_UNUSED lower(Value v){char*s=text(v);for(char*p=s;*p;p++)*p=(char)tolower((unsigned char)*p);return str(s);}
static Value RUBYC_UNUSED chomp(Value v){char*s=text(v);size_t n=strlen(s);while(n&& (s[n-1]=='\n'||s[n-1]=='\r'))s[--n]=0;return str(s);}
static Value RUBYC_UNUSED input(void){char b[4096];return fgets(b,sizeof b,stdin)?str(dup(b)):nil();}
`
