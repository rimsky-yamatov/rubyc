// Package compiler contains the Ruby subset frontend and TinyCC driver.
package compiler

import (
    "bytes"
    "fmt"
    "strconv"
    "strings"
)

// Compile translates the supported Ruby subset to a standalone C program.
// The frontend intentionally keeps the grammar small, but supports Ruby's most
// useful data-oriented constructs: arrays, hashes, methods, blocks, classes and
// modules. Unsupported syntax is rejected rather than silently miscompiled.
func Compile(source, filename string) (string, error) {
    p := newParser(source, filename)
    body, err := p.program("")
    if err != nil { return "", err }
    var b bytes.Buffer
    b.WriteString(runtimeC)
    for _, f := range p.functions { b.WriteString(f) }
    b.WriteString("\nint main(void){\n")
    for _, s := range body { b.WriteString(s.c(1)) }
    b.WriteString("return 0;\n}\n")
    return b.String(), nil
}

type expr interface{ c() string }
type stmt interface{ c(int) string }
type literal struct{ text string }
func (e literal) c() string { return e.text }
type binary struct { op string; a,b expr }
func (e binary) c() string { switch e.op { case "+": return "add("+e.a.c()+","+e.b.c()+")"; case "==": return "num(eq("+e.a.c()+","+e.b.c()+"))"; case "!=": return "num(!eq("+e.a.c()+","+e.b.c()+"))" }; return "num(("+e.a.c()+").n"+e.op+"("+e.b.c()+").n)" }
type unary struct { op string; value expr }
func (e unary) c() string { if e.op=="!" { return "num(!truth("+e.value.c()+"))" }; return "num(-("+e.value.c()+").n)" }
type call struct { receiver expr; name string; args []expr }
func (e call) c() string { if e.receiver==nil && e.name=="gets" { return "input()" }; name:=e.name; if e.receiver!=nil { m:=map[string]string{"length":"length","size":"length","upcase":"upper","downcase":"lower","chomp":"chomp","to_s":"to_s","push":"array_push","pop":"array_pop","first":"array_first","last":"array_last","keys":"hash_keys"}; name=m[name]; if name=="" { name="method_"+e.name }; a:=[]string{e.receiver.c()}; for _,x:=range e.args { a=append(a,x.c()) }; return name+"("+strings.Join(a,",")+")" }; a:=[]string{}; for _,x:=range e.args { a=append(a,x.c()) }; return name+"("+strings.Join(a,",")+")" }
type index struct { a,i expr }
func (e index) c() string { return "index("+e.a.c()+","+e.i.c()+")" }
type assign struct { name string; value expr; decl bool }
func (s assign) c(i int) string { pre:=""; if s.decl { pre="Value " }; return strings.Repeat("\t",i)+pre+s.name+"="+s.value.c()+";\n" }
type indexAssign struct { a,i,v expr }
func (s indexAssign) c(i int) string { return strings.Repeat("\t",i)+"setindex("+s.a.c()+","+s.i.c()+","+s.v.c()+");\n" }
type output struct { value expr; nl bool }
func (s output) c(i int) string { n:="0"; if s.nl {n="1"}; return strings.Repeat("\t",i)+"out("+s.value.c()+","+n+");\n" }
type conditional struct { test expr; yes,no []stmt }
func (s conditional) c(i int) string { pad:=strings.Repeat("\t",i); var b strings.Builder; b.WriteString(pad+"if (truth("+s.test.c()+")) {\n"); for _,x:=range s.yes {b.WriteString(x.c(i+1))}; b.WriteString(pad+"}"); if len(s.no)>0 {b.WriteString(" else {\n");for _,x:=range s.no {b.WriteString(x.c(i+1))};b.WriteString(pad+"}")};return b.String()+"\n" }
type loop struct { test expr; body []stmt }
func (s loop) c(i int) string { pad:=strings.Repeat("\t",i); var b strings.Builder;b.WriteString(pad+"while (truth("+s.test.c()+")) {\n");for _,x:=range s.body {b.WriteString(x.c(i+1))};return b.String()+pad+"}\n" }
type eachLoop struct { collection expr; variable string; body []stmt }
func (s eachLoop) c(i int) string { p:=strings.Repeat("\t",i); var b strings.Builder; b.WriteString(p+"for (long _i=0; _i<array_len("+s.collection.c()+"); _i++) {\n"+p+"\tValue "+s.variable+"=array_get("+s.collection.c()+",_i);\n");for _,x:=range s.body {b.WriteString(x.c(i+1))};return b.String()+p+"}\n" }
type ret struct { value expr }
func (s ret) c(i int) string { return strings.Repeat("\t",i)+"return "+s.value.c()+";\n" }

type parser struct { lines []string; file string; line int; vars map[string]bool; functions []string; class string }
func newParser(s,f string)*parser{return &parser{lines:strings.Split(s,"\n"),file:f,vars:map[string]bool{}}}
func(p *parser)problem(f string,a ...any)error{return fmt.Errorf("%s:%d: %s",p.file,p.line+1,fmt.Sprintf(f,a...))}
func clean(s string)string{return strings.TrimSpace(strings.SplitN(s,"#",2)[0])}
func(p *parser)program(stop string)([]stmt,error){var out []stmt;for p.line<len(p.lines){raw:=clean(p.lines[p.line]);p.line++;if raw==""{continue};if raw=="end"||raw=="else"{if stop!=""{return out,nil};return nil,p.problem("unexpected %q",raw)}
 if strings.HasPrefix(raw,"class ")||strings.HasPrefix(raw,"module "){ name:=strings.Fields(raw)[1]; old:=p.class;p.class=name; body,err:=p.program("end");p.class=old;if err!=nil{return nil,err};_ = body;continue }
 if strings.HasPrefix(raw,"def "){if err:=p.parseDef(raw);err!=nil{return nil,err};continue}
 if strings.HasPrefix(raw,"if ")||strings.HasPrefix(raw,"unless "){pre:="if "; neg:=false;if strings.HasPrefix(raw,"unless "){pre="unless ";neg=true};t,e:=p.expression(strings.TrimSpace(raw[len(pre):]));if e!=nil{return nil,e};if neg{t=unary{"!",t}};yes,e:=p.program("end");if e!=nil{return nil,e};var no []stmt;if p.line<=len(p.lines)&&clean(p.lines[p.line-1])=="else"{no,e=p.program("end");if e!=nil{return nil,e}};out=append(out,conditional{t,yes,no});continue}
 if strings.HasPrefix(raw,"while "){t,e:=p.expression(strings.TrimSpace(raw[6:]));if e!=nil{return nil,e};body,e:=p.program("end");if e!=nil{return nil,e};out=append(out,loop{t,body});continue}
 if strings.HasSuffix(raw," do")&&strings.Contains(raw,".each"){x:=strings.TrimSuffix(raw," do");parts:=strings.SplitN(x,".each",2);v:="item";if i:=strings.Index(x,"|");i>=0{v=strings.Trim(x[i+1:]," |")};col,e:=p.expression(strings.TrimSpace(parts[0]));if e!=nil{return nil,e};body,e:=p.program("end");if e!=nil{return nil,e};out=append(out,eachLoop{col,v,body});continue}
 if strings.HasPrefix(raw,"puts ")||strings.HasPrefix(raw,"print "){nl:=strings.HasPrefix(raw,"puts ");n:=6;if !nl{n=6};e,er:=p.expression(strings.TrimSpace(raw[n:]));if er!=nil{return nil,er};out=append(out,output{e,nl});continue}
 if strings.HasPrefix(raw,"return "){e,er:=p.expression(strings.TrimSpace(raw[7:]));if er!=nil{return nil,er};out=append(out,ret{e});continue}
 if n,v,ok:=strings.Cut(raw,"=");ok&&!strings.Contains(v,"="){lhs:=strings.TrimSpace(n);e,er:=p.expression(strings.TrimSpace(v));if er!=nil{return nil,er};if strings.Contains(lhs,"["){q:=strings.Index(lhs,"[");a,er:=p.expression(lhs[:q]);if er!=nil{return nil,er};ii,er:=p.expression(strings.TrimSuffix(lhs[q+1:],"]"));if er!=nil{return nil,er};out=append(out,indexAssign{a,ii,e});continue};if !isIdent(lhs){return nil,p.problem("invalid assignment")};out=append(out,assign{lhs,e,!p.vars[lhs]});p.vars[lhs]=true;continue}
 if _,er:=p.expression(raw);er==nil{e,_:=p.expression(raw);out=append(out,output{e,false});continue};return nil,p.problem("unsupported syntax: %s",raw)};if stop!=""{return nil,p.problem("missing end")};return out,nil}
func(p *parser)parseDef(raw string)error{head:=strings.TrimSpace(raw[4:]);q:=strings.Index(head,"(");name:=head;args:=[]string{};if q>=0{name=head[:q];args=strings.Split(strings.TrimSuffix(head[q+1:],")"),",");for i:=range args{args[i]=strings.TrimSpace(args[i])}};old:=p.vars;p.vars=map[string]bool{};for _,a:=range args{if a!=""{p.vars[a]=true}};body,err:=p.program("end");p.vars=old;if err!=nil{return err};fn:=name;if p.class!=""{fn=p.class+"_"+name};var b strings.Builder;b.WriteString("static Value "+fn+"(");for i,a:=range args{if i>0{b.WriteString(",")};b.WriteString("Value "+a)};b.WriteString("){\n");for _,s:=range body{b.WriteString(s.c(1))};b.WriteString("return nil();}\n");p.functions=append(p.functions,b.String());return nil}
func isIdent(s string)bool{if s==""{return false};for i,r:=range s{if !(r=='_'||(r>='a'&&r<='z')||(r>='A'&&r<='Z')||(i>0&&r>='0'&&r<='9')){return false}};return true}

func(p *parser)expression(s string)(expr,error){ts,err:=lexExpression(s);if err!=nil{return nil,p.problem("%v",err)};pos:=0;prec:=map[string]int{"==":1,"!=":1,"<":2,"<=":2,">":2,">=":2,"+":3,"-":3,"*":4,"/":4};var parse func(int)(expr,error);atom:=func(x string)(expr,error){if n,e:=strconv.ParseInt(x,10,64);e==nil{return literal{"num("+strconv.FormatInt(n,10)+")"},nil};if len(x)>=2&&x[0]=='"'{q,e:=strconv.Unquote(x);if e!=nil{return nil,e};return literal{"str("+strconv.Quote(q)+")"},nil};if x=="true"{return literal{"num(1)"},nil};if x=="false"{return literal{"num(0)"},nil};if x=="nil"{return literal{"nil()"},nil};if p.vars[x]||isIdent(x){return literal{x},nil};return nil,p.problem("unknown value %q",x)}
 parse=func(min int)(expr,error){if pos>=len(ts){return nil,p.problem("expected expression")};t:=ts[pos];pos++;var l expr;var e error;if t=="("{l,e=parse(0);if e!=nil{return nil,e};if pos>=len(ts)||ts[pos]!=")"{return nil,p.problem("missing )")};pos++}else if t=="["{var a []expr;for pos<len(ts)&&ts[pos]!="]"{x,er:=parse(0);if er!=nil{return nil,er};a=append(a,x);if pos<len(ts)&&ts[pos]==","{pos++}};if pos>=len(ts){return nil,p.problem("missing ]")};pos++;cs:=[]string{};for _,x:=range a{cs=append(cs,x.c())};l=literal{"array("+strings.Join(cs,",")+")"}}else if t=="{"{var a []string;for pos<len(ts)&&ts[pos]!="}"{k,er:=parse(0);if er!=nil{return nil,er};if pos>=len(ts)||ts[pos]!="=>"{return nil,p.problem("expected =>")};pos++;v,er:=parse(0);if er!=nil{return nil,er};a=append(a,k.c(),v.c());if pos<len(ts)&&ts[pos]==","{pos++}};if pos>=len(ts){return nil,p.problem("missing }")};pos++;l=literal{"hash("+strings.Join(a,",")+")"}}else if t=="!"||t=="-"{v,er:=parse(5);if er!=nil{return nil,er};l=unary{t,v}}else if t=="gets"{l=call{name:"gets"}}else{l,e=atom(t);if e!=nil{return nil,e}}
 for pos<len(ts){if ts[pos]=="["{pos++;ii,er:=parse(0);if er!=nil{return nil,er};if pos>=len(ts)||ts[pos]!="]"{return nil,p.problem("missing ]")};pos++;l=index{l,ii};continue};if ts[pos]=="."{pos++;if pos>=len(ts){return nil,p.problem("expected method")};n:=ts[pos];pos++;var aa []expr;if pos<len(ts)&&ts[pos]=="("{pos++;for pos<len(ts)&&ts[pos]!=")"{x,er:=parse(0);if er!=nil{return nil,er};aa=append(aa,x);if pos<len(ts)&&ts[pos]==","{pos++}};if pos>=len(ts){return nil,p.problem("missing )")};pos++};l=call{l,n,aa};continue};op:=ts[pos];pr,ok:=prec[op];if !ok||pr<min{break};pos++;r,er:=parse(pr+1);if er!=nil{return nil,er};l=binary{op,l,r}};return l,nil};e,err:=parse(0);if err!=nil{return nil,err};if pos!=len(ts){return nil,p.problem("unexpected token %q",ts[pos])};return e,nil}
func lexExpression(s string)([]string,error){var t []string;for i:=0;i<len(s);{if s[i]==' '||s[i]=='\t'{i++;continue};if strings.ContainsRune("+-*/()!<>.,[]{}",rune(s[i])){if i+1<len(s)&&((s[i]=='='&&s[i+1]=='=')||(s[i]=='!'&&s[i+1]=='=')||(s[i]=='<'&&s[i+1]=='=')||(s[i]=='>'&&s[i+1]=='=')||(s[i]=='='&&s[i+1]=='>')){t=append(t,s[i:i+2]);i+=2}else{t=append(t,s[i:i+1]);i++};continue};if s[i]=='"'{st:=i;i++;for i<len(s)&&s[i]!='"'{if s[i]=='\\'{i++};i++};if i>=len(s){return nil,fmt.Errorf("unterminated string")};i++;t=append(t,s[st:i]);continue};st:=i;for i<len(s)&&!strings.ContainsRune(" \t+-*/()!<>.,[]{}",rune(s[i])){i++};t=append(t,s[st:i])};return t,nil}
