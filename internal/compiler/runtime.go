package compiler

// runtimeC is deliberately kept separate from the frontend so it can grow into
// a versioned Ruby runtime without obscuring parsing and lowering code.
const runtimeC = `#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <ctype.h>
#include <stdarg.h>

#if defined(__GNUC__) || defined(__TINYC__)
#define RUBYC_UNUSED __attribute__((unused))
#else
#define RUBYC_UNUSED
#endif

typedef struct { int kind; long n; char *s; } Value;
typedef struct { long len; Value *items; } Array;
typedef struct { long len; Value *keys; Value *values; } Hash;

static Value RUBYC_UNUSED num(long n){return (Value){0,n,0};}
static Value str(char*s){return (Value){1,0,s};}
static Value nil(void){return (Value){2,0,0};}
static Value array_of(long len, ...){
    Array *a = malloc(sizeof(*a));
    a->len = len;
    a->items = malloc(sizeof(Value) * (size_t)len);
    va_list ap;
    va_start(ap, len);
    for (long i = 0; i < len; i++) {
        a->items[i] = va_arg(ap, Value);
    }
    va_end(ap);
    return (Value){3,0,(char *)a};
}
static Value hash_of(long len, ...){
    Hash *h = malloc(sizeof(*h));
    h->len = len;
    h->keys = malloc(sizeof(Value) * (size_t)len);
    h->values = malloc(sizeof(Value) * (size_t)len);
    va_list ap;
    va_start(ap, len);
    for (long i = 0; i < len; i++) {
        h->keys[i] = va_arg(ap, Value);
        h->values[i] = va_arg(ap, Value);
    }
    va_end(ap);
    return (Value){4,0,(char *)h};
}
static char *dup(const char*s){char*r=malloc(strlen(s)+1);strcpy(r,s);return r;}
static int RUBYC_UNUSED truth(Value v){return v.kind != 2 && (v.kind != 0 || v.n != 0) && (v.kind != 3 || v.s != 0) && (v.kind != 4 || v.s != 0);}
static char *text(Value v){char b[64];if(v.kind==1)return dup(v.s);if(v.kind==2)return dup("");if(v.kind==3)return dup("[array]");if(v.kind==4)return dup("{hash}");snprintf(b,sizeof b,"%ld",v.n);return dup(b);}
static void out(Value v,int nl){if(v.kind==1)fputs(v.s,stdout);else if(v.kind==2)fputs("nil",stdout);else if(v.kind==3){Array *a=(Array *)v.s;fputs("[",stdout);for(long i=0;i<a->len;i++){if(i)fputs(", ",stdout);out(a->items[i],0);}fputs("]",stdout);}else if(v.kind==4){Hash *h=(Hash *)v.s;fputs("{",stdout);for(long i=0;i<h->len;i++){if(i)fputs(", ",stdout);out(h->keys[i],0);fputs("=>",stdout);out(h->values[i],0);}fputs("}",stdout);}else printf("%ld",v.n);if(nl)putchar('\n');}
static Value RUBYC_UNUSED add(Value a,Value b){if(!a.kind&&!b.kind)return num(a.n+b.n);char *as=text(a),*bs=text(b),*r=malloc(strlen(as)+strlen(bs)+1);strcpy(r,as);strcat(r,bs);return str(r);}
static int RUBYC_UNUSED eq(Value a,Value b){if(a.kind!=b.kind)return 0;return a.kind==1?strcmp(a.s,b.s)==0:a.kind==2||a.n==b.n;}
static Value RUBYC_UNUSED length(Value v){if(v.kind==3)return num(((Array *)v.s)->len);if(v.kind==4)return num(((Hash *)v.s)->len);return num((long)strlen(text(v)));}
static Value RUBYC_UNUSED upper(Value v){char*s=text(v);for(char*p=s;*p;p++)*p=(char)toupper((unsigned char)*p);return str(s);}
static Value RUBYC_UNUSED lower(Value v){char*s=text(v);for(char*p=s;*p;p++)*p=(char)tolower((unsigned char)*p);return str(s);}
static Value RUBYC_UNUSED chomp(Value v){char*s=text(v);size_t n=strlen(s);while(n&& (s[n-1]=='\n'||s[n-1]=='\r'))s[--n]=0;return str(s);}
static Value RUBYC_UNUSED input(void){char b[4096];return fgets(b,sizeof b,stdin)?str(dup(b)):nil();}
static Value index(Value a, Value idx){
    if (a.kind == 3) {
        Array *arr = (Array *)a.s;
        long i = idx.n;
        if (i < 0 || i >= arr->len) return nil();
        return arr->items[i];
    }
    if (a.kind == 4) {
        Hash *h = (Hash *)a.s;
        if (idx.kind != 1) return nil();
        for (long i = 0; i < h->len; i++) {
            if (h->keys[i].kind == 1 && strcmp(h->keys[i].s, idx.s) == 0) {
                return h->values[i];
            }
        }
        return nil();
    }
    return nil();
}
static void setindex(Value a, Value idx, Value val){
    if (a.kind == 3) {
        Array *arr = (Array *)a.s;
        long i = idx.n;
        if (i >= 0 && i < arr->len) { arr->items[i] = val; }
        return;
    }
    if (a.kind == 4) {
        Hash *h = (Hash *)a.s;
        if (idx.kind != 1) return;
        for (long i = 0; i < h->len; i++) {
            if (h->keys[i].kind == 1 && strcmp(h->keys[i].s, idx.s) == 0) {
                h->values[i] = val;
                return;
            }
        }
        h->keys = realloc(h->keys, sizeof(Value) * (size_t)(h->len + 1));
        h->values = realloc(h->values, sizeof(Value) * (size_t)(h->len + 1));
        h->keys[h->len] = idx;
        h->values[h->len] = val;
        h->len++;
    }
}
`
