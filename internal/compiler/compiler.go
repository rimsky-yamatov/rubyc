package compiler

import (
 "bytes"
 "fmt"
 "strconv"
 "strings"
)

func Compile(source, filename string) (string,error) {
 p:=newParser(source,filename); body,err:=p.program(""); if err!=nil{return "",err}
 var b bytes.Buffer; b.WriteString(runtimeC); b.WriteByte('\n')
 for _,f:=range p.functions { b.WriteString(f) }
 b.WriteString("int main(void){\n"); for _,s:=range body {b.WriteString(s.c(1))}; b.WriteString("return 0;\n}\n"); return b.String(),nil
}

type expr interface{c()string}; type stmt interface{c(int)string}
type literal struct{text string}; func(e literal)c()string{return e.text}
type binary struct{op string;a,b expr}; func(e binary)c()string{switch e.op{case "+":return "add("+e.a.c()+","+e.b.c()+")";case "==":return "num(eq("+e.a.c()+","+e.b.c()+"))";case "!=":return "num(!eq("+e.a.c()+","+e.b.c()+"))"};return "num(("+e.a.c()+").n"+e.op+"("+e.b.c()+").n)"}
type unary struct{op string;value expr};func(e unary)c()string{if e.op=="!"{return "num(!truth("+e.value.c()+"))"};return "num(-("+e.value.c()+").n)"}
type call struct{receiver expr;name string;args []expr};func(e call)c()string{if e.receiver==nil&&e.name=="gets"{return "input()"};if e.receiver!=nil{switch e.name{case "to_s":return "str(text("+e.receiver.c()+"))";case "length","size":return "length("+e.receiver.c()+")";case "upcase":return "upper("+e.receiver.c()+")";case "downcase":return "lower("+e.receiver.c()+")";case "chomp":return "chomp("+e.receiver.c()+")"};};a:=[]string{};if e.receiver!=nil{a=append(a,e.receiver.c())};for _,x:=range e.args{a=append(a,x.c())};return e.name+"("+strings.Join(a,",")+")"}
type index struct{a,i expr};func(e index)c()string{return "index("+e.a.c()+","+e.i.c()+")"}
type assign struct{name string;value expr;decl bool};func(s assign)c(i int)string{d:="";if s.decl{d="Value "};return strings.Repeat("\t",i)+d+s.name+"="+s.value.c()+";\n"}
type indexAssign struct{a,i,v expr};func(s indexAssign)c(i int)string{return strings.Repeat("\t",i)+"setindex("+s.a.c()+","+s.i.c()+","+s.v.c()+");\n"}
type output struct{value expr;nl bool};func(s output)c(i int)string{n:="0";if s.nl{n="1"};return strings.Repeat("\t",i)+"out("+s.value.c()+","+n+");\n"}
type conditional struct{test expr;yes,no []stmt};func(s conditional)c(i int)string{p:=strings.Repeat("\t",i);var b strings.Builder;b.WriteString(p+"if (truth("+s.test.c()+")) {\n");for _,x:=range s.yes{b.WriteString(x.c(i+1))};b.WriteString(p+"}");if len(s.no)>0{b.WriteString(" else {\n");for _,x:=range s.no{b.WriteString(x.c(i+1))};b.WriteString(p+"}")};return b.String()+"\n"}
type loop struct{test expr;body []stmt};func(s loop)c(i int)string{p:=strings.Repeat("\t",i);var b strings.Builder;b.WriteString(p+"while (truth("+s.test.c()+")) {\n");for _,x:=range s.body{b.WriteString(x.c(i+1))};return b.String()+p+"}\n"}
type returnStmt struct{value expr};func(s returnStmt)c(i int)string{return strings.Repeat("\t",i)+"return "+s.value.c()+";\n"}
type method struct{name string;args []string;body []stmt};func(s method)c()string{var b strings.Builder;b.WriteString("static Value "+s.name+"(");for i,a:=range s.args{if i>0{b.WriteString(",")};b.WriteString("Value "+a)};b.WriteString("){\n");for _,x:=range s.body{b.WriteString(x.c(1))};b.WriteString("return nil();\n}\n");return b.String()}

type parser struct{lines []string;file string;line int;vars map[string]bool;functions []string;methods map[string]bool;inMethod bool}
func newParser(s,f string)*parser{return &parser{lines:strings.Split(s,"\n"),file:f,vars:map[string]bool{},methods:map[string]bool{}}}
func(p *parser)problem(f string,a ...any)error{return fmt.Errorf("%s:%d: %s",p.file,p.line+1,fmt.Sprintf(f,a...))}
func clean(s string)string{return strings.TrimSpace(strings.SplitN(s,"#",2)[0])}
func(p *parser)program(stop string)([]stmt,error){var out []stmt;for p.line<len(p.lines){r:=clean(p.lines[p.line]);p.line++;if r==""{continue};if r=="end"||r=="else"{if stop!=""{return out,nil};return nil,p.problem("unexpected %q",r)}
 if strings.HasPrefix(r,"def "){if err:=p.parseMethod(r);err!=nil{return nil,err};continue}
 if strings.HasPrefix(r,"if ")||strings.HasPrefix(r,"unless "){neg:=strings.HasPrefix(r,"unless ");prefix:="if ";if neg{prefix="unless "};t,e:=p.expression(strings.TrimSpace(r[len(prefix):]));if e!=nil{return nil,e};if neg{t=unary{"!",t}};yes,e:=p.program("end");if e!=nil{return nil,e};var no []stmt;if p.line<=len(p.lines)&&clean(p.lines[p.line-1])=="else"{no,e=p.program("end");if e!=nil{return nil,e}};if p.line>len(p.lines)||clean(p.lines[p.line-1])!="end"{return nil,p.problem("if without matching end")};out=append(out,conditional{t,yes,no});continue}
 if strings.HasPrefix(r,"while "){t,e:=p.expression(strings.TrimSpace(r[6:]));if e!=nil{return nil,e};body,e:=p.program("end");if e!=nil{return nil,e};out=append(out,loop{t,body});continue}
 if strings.HasPrefix(r,"puts ")||strings.HasPrefix(r,"print "){nl:=strings.HasPrefix(r,"puts ");n:=6;e,er:=p.expression(strings.TrimSpace(r[n:]));if er!=nil{return nil,er};out=append(out,output{e,nl});continue}
 if strings.HasPrefix(r,"return "){e,er:=p.expression(strings.TrimSpace(r[7:]));if er!=nil{return nil,er};out=append(out,returnStmt{e});continue}
 if lhs,rhs,ok:=strings.Cut(r,"=");ok&&!strings.Contains(rhs,"="){lhs=strings.TrimSpace(lhs);rhs=strings.TrimSpace(rhs);if strings.Contains(lhs,"["){q:=strings.Index(lhs,"[");z:=strings.LastIndex(lhs,"]");a,e:=p.expression(lhs[:q]);if e!=nil{return nil,e};i,e:=p.expression(lhs[q+1:z]);if e!=nil{return nil,e};v,e:=p.expression(rhs);if e!=nil{return nil,e};out=append(out,indexAssign{a,i,v});continue};if !isIdent(lhs){return nil,p.problem("invalid assignment")};v,e:=p.expression(rhs);if e!=nil{return nil,e};out=append(out,assign{lhs,v,!p.vars[lhs]});p.vars[lhs]=true;continue}
 if e,eerr:=p.expression(r);eerr==nil{out=append(out,output{e,false});continue};return nil,p.problem("unsupported syntax: %s",r)};if stop!=""{return nil,p.problem("missing end")};return out,nil}
func(p *parser)parseMethod(r string)error{h:=strings.TrimSpace(r[4:]);q:=strings.Index(h,"(");name:=h;var args []string;if q>=0{name=h[:q];z:=strings.LastIndex(h,")");if z<q{return p.problem("invalid method declaration")};if strings.TrimSpace(h[q+1:z])!=""{for _,a:=range strings.Split(h[q+1:z],","){a=strings.TrimSpace(a);if !isIdent(a){return p.problem("invalid argument %q",a)};args=append(args,a)}}};old:=p.vars;p.vars=map[string]bool{};for _,a:=range args{p.vars[a]=true};oldIn:=p.inMethod;p.inMethod=true;body,e:=p.program("end");p.inMethod=oldIn;p.vars=old;if e!=nil{return e};m:=method{name,args,body};p.functions=append(p.functions,m.c());p.methods[name]=true;return nil}
func isIdent(s string)bool{if s==""{return false};for i,r:=range s{if !(r=='_'||(r>='a'&&r<='z')||(r>='A'&&r<='Z')||(i>0&&r>='0'&&r<='9')){return false}};return true}

func(p *parser)expression(s string)(expr,error){ts,e:=lexExpression(s);if e!=nil{return nil,p.problem("%v",e)};pos:=0;prec:=map[string]int{"==":1,"!=":1,"<":2,"<=":2,">":2,">=":2,"+":3,"-":3,"*":4,"/":4};var parse func(int)(expr,error);atom:=func(x string)(expr,error){if n,e:=strconv.ParseInt(x,10,64);e==nil{return literal{"num("+strconv.FormatInt(n,10)+")"},nil};if len(x)>1&&x[0]=='"'{q,e:=strconv.Unquote(x);if e!=nil{return nil,e};return literal{"str("+strconv.Quote(q)+")"},nil};switch x{case "true":return literal{"num(1)"},nil;case "false":return literal{"num(0)"},nil;case "nil":return literal{"nil()"},nil};if p.vars[x]||p.methods[x]||isIdent(x){return literal{x},nil};return nil,p.problem("unknown value %q",x)}
 parse=func(min int)(expr,error){if pos>=len(ts){return nil,p.problem("expected expression")};t:=ts[pos];pos++;var l expr;var er error;switch t{case "(":l,er=parse(0);if er!=nil{return nil,er};if pos>=len(ts)||ts[pos]!=")"{return nil,p.problem("missing )")};pos++;case "[":var a []expr;for pos<len(ts)&&ts[pos]!="]"{x,e:=parse(0);if e!=nil{return nil,e};a=append(a,x);if pos<len(ts)&&ts[pos]==","{pos++}};if pos>=len(ts){return nil,p.problem("missing ]")};pos++;aa:=[]string{strconv.Itoa(len(a))};for _,x:=range a{aa=append(aa,x.c())};l=literal{"array_of(num("+aa[0]+"),"+strings.Join(aa[1:],",")+")"};case "!","-":x,e:=parse(5);if e!=nil{return nil,e};l=unary{t,x};case "gets":l=call{name:"gets"};default:l,er=atom(t);if er!=nil{return nil,er}}
 for pos<len(ts){if ts[pos]=="["{pos++;i,e:=parse(0);if e!=nil{return nil,e};if pos>=len(ts)||ts[pos]!="]"{return nil,p.problem("missing ]")};pos++;l=index{l,i};continue};if ts[pos]=="."{pos++;if pos>=len(ts){return nil,p.problem("expected method")};n:=ts[pos];pos++;var a []expr;if pos<len(ts)&&ts[pos]=="("{pos++;for pos<len(ts)&&ts[pos]!=")"{x,e:=parse(0);if e!=nil{return nil,e};a=append(a,x);if pos<len(ts)&&ts[pos]==","{pos++}};if pos>=len(ts){return nil,p.problem("missing )")};pos++};l=call{l,n,a};continue};op:=ts[pos];pr,ok:=prec[op];if !ok||pr<min{break};pos++;r,e:=parse(pr+1);if e!=nil{return nil,e};l=binary{op,l,r}};return l,nil};x,e:=parse(0);if e!=nil{return nil,e};if pos!=len(ts){return nil,p.problem("unexpected token %q",ts[pos])};return x,nil}
func lexExpression(s string)([]string,error){var t []string;for i:=0;i<len(s);{if s[i]==' '||s[i]=='\t'{i++;continue};if i+1<len(s)&&s[i]=='='&&s[i+1]=='>'{t=append(t,"=>");i+=2;continue};if strings.ContainsRune("+-*/()!<>.,[]{}",rune(s[i])){if i+1<len(s)&&strings.ContainsRune("=!<>",rune(s[i]))&&s[i+1]=='='{t=append(t,s[i:i+2]);i+=2}else{t=append(t,s[i:i+1]);i++};continue};if s[i]=='"'{st:=i;i++;esc:=false;closed:=false;for i<len(s){if s[i]=='"'&&!esc{i++;closed=true;break};esc=s[i]=='\\'&&!esc;if s[i]!='\\'{esc=false};i++};if !closed{return nil,fmt.Errorf("unterminated string")};t=append(t,s[st:i]);continue};st:=i;for i<len(s)&&s[i]!=' '&&s[i]!='\t'&&!strings.ContainsRune("+-*/()!<>.,[]{}",rune(s[i])){i++};t=append(t,s[st:i])};return t,nil}
