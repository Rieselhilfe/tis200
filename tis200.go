package main

import (
    "fmt"
    "io/ioutil"
    "strings"
    "strconv"
)

const NOP,MOV,SWP,SAV,ADD,SUB,NEG,JMP,JEZ,JNZ,JGZ,JLZ,JRO = 1,2,3,4,5,6,7,8,9,10,11,12,13
const ACC,ANY,LAST,NIL,UP,RIGHT,DOWN,LEFT = 1,2,3,4,5,6,7,8
const dirs = 4

var cmd_codes = map[string]byte{
    "NOP":      NOP,
    "MOV":      MOV,
    "SWP":      SWP,
    "SAV":      SAV,
    "ADD":      ADD,
    "SUB":      SUB,
    "NEG":      NEG,
    "JMP":      JMP,
    "JEZ":      JEZ,
    "JNZ":      JNZ,
    "JGZ":      JGZ,
    "JLZ":      JLZ,
    "JRO":      JRO,
}

var reg_codes = map[string]int{
    "ACC":      ACC,
    "ANY":      ANY,
    "LAST":     LAST,
    "NIL":      NIL,
    "UP":       UP,
    "LEFT":     LEFT,
    "RIGHT":    RIGHT,
    "DOWN":     DOWN, //TODO more
}

type cmd struct {
    code byte //cmd_code
    arg1 int //arg1
    arg2 int //arg2
    mode byte //ll,lr,rl,rr: binary represantions of argtype: lexp or rexp
}

type edge struct {
    dest *node //index of node
    incoming int //incoming value
    written bool //is the value written by another node, that is waiting for this one to read it?
}

type node struct {
    code []cmd //slice of cmds: Code stored in the node
    pc int //program counter
    acc int //acc-register
    bak int //bak-register
    edges []edge //edges of the node
    waiting bool //is this node waiting for a read from another node?
    to_write *edge //pointer to edge to write to
}

func (n *node) get_reg(reg_code byte) (succ bool, val int) { //TODO ANY and LAST
    switch {
    case reg_code == ACC:
        return true, n.acc

    case reg_code == ANY:
        for i, ed := range n.edges {
            if ed.written {
                n.edges[i].written = false
                n.edges[i].dest.waiting = false
                return true, ed.incoming
            }
        }
    case reg_code >= UP:
        if n.edges[reg_code-5].written {
            n.edges[reg_code-5].written = false
            if (n.edges[reg_code-5].dest != nil) {
                n.edges[reg_code-5].dest.waiting = false
            }
            return true, n.edges[reg_code-5].incoming
        }
    }
    return false, 0
}

func (n *node) set_reg(val int, reg_code byte) bool { //TODO ANY and LAST
    switch {
    case reg_code == ACC:
        n.acc = val
    case reg_code >= UP:
        n.edges[reg_code-5].dest.edges[(reg_code-5+dirs/2)%dirs].incoming = val
        n.to_write = &n.edges[reg_code-5].dest.edges[(reg_code-5+dirs/2)%dirs]
        n.waiting = true
        return true
    }
    return false
}

func (n *node) write() {
    if n.to_write != nil {
        n.to_write.written = true
    }
}

func (n *node) tick() (wants_to_write bool) {
    if (n.waiting) { //if the node is waiting on a communication
        return false //then dont execute a function, just break
    }
    switch command := n.code[n.pc]; command.code {
    case MOV:
        if command.mode == 1 {
            wants_to_write = n.set_reg(command.arg1,byte(command.arg2))
        } else if command.mode == 3 {
            if succ, val := n.get_reg(byte(command.arg1)); succ {
                wants_to_write = n.set_reg(val, byte(command.arg2))
            } else {
                n.pc -= 1
            }
        } else {
            fmt.Println("wrong mov mode",command.mode)
        }
    case SWP:
        n.acc, n.bak = n.bak, n.acc
    case SAV:
        n.bak = n.acc
    case ADD:
        if command.mode < 2 { //wenn arg1 lexp ist
            n.acc += command.arg1
        } else {
            if succ, val := n.get_reg(byte(command.arg1)); succ {
                n.acc += val
            } else {
                n.pc -= 1
            }
        }
    case SUB:
        if command.mode < 2 { //wenn arg1 lexp ist
            n.acc -= command.arg1
        } else {
            if succ, val := n.get_reg(byte(command.arg1)); succ {
                n.acc -= val
            } else {
                n.pc -= 1
            }
        }
    case NEG:
        n.acc = -n.acc
    case JMP:
        n.pc = command.arg1-1
    case JEZ:
        if n.acc==0 {
            n.pc = command.arg1-1
        }
    case JNZ:
        if n.acc!=0 {
            n.pc = command.arg1-1
        }
    case JGZ:
        if n.acc>0 {
            n.pc = command.arg1-1
        }
    case JLZ:
        if n.acc<0 {
            n.pc = command.arg1-1
        }
    case JRO:
        n.pc = (n.pc + command.arg1)%len(n.code)-1
    default:
        fmt.Println("unknown command code",command.code)
    }
    n.pc += 1
    if n.code[n.pc].code==0 {n.pc=0}
    return wants_to_write
}

type inp_node struct {
    values []int
    edges []edge
    vc int
    waiting bool
}

type tis struct { //struct for the whole tesselated intelligence system
    loc int
    inp_nodes []inp_node
    nodes []node
    inp_amount int
    outp_amount int
}

func (t *tis) tick() { //method that gets called every tick
    for j, nod := range t.inp_nodes {
        for k, ed := range nod.edges {
            if ed.dest != nil{
                if !ed.dest.edges[(k+dirs/2)%dirs].written && nod.vc < len(t.inp_nodes[j].values) {
                    ed.dest.edges[(k+dirs/2)%dirs].incoming = nod.values[nod.vc]
                    ed.dest.edges[(k+dirs/2)%dirs].written = true
                    t.inp_nodes[j].waiting = true
                    t.inp_nodes[j].vc += 1
                }
            }
        }
    }
    for j, nod := range t.nodes {
         if j<t.outp_amount{
            for k, ed := range nod.edges {
                if ed.dest != nil {
                    if ed.written {
                        t.nodes[j].edges[k].written = false
                        t.nodes[j].edges[k].dest.waiting = false
                        fmt.Println("out",ed.incoming)
                    }
                }
            }
        } else if t.nodes[j].tick() { //if the node wants to write
            defer t.nodes[j].write() //let it write later
        }
    }
}

func (t *tis) run() {
    for i:=0;i<100;i++ {
        t.tick()
    }
}

func construct_tis(source string, code string) (ret_tis tis) { //produces the tis-graph
    text := strings.Split(strings.Trim(source,"\n"),"\n")
    base_config := strings.Split(text[0], " ")
    ret_tis.loc, _ = strconv.Atoi(base_config[0])
    ret_tis.inp_amount, _ = strconv.Atoi(base_config[1])
    ret_tis.outp_amount, _ = strconv.Atoi(base_config[2])
    ret_tis.inp_nodes = make([]inp_node, len(text[1:ret_tis.inp_amount+1]))
    ret_tis.nodes = make([]node, len(text[1+ret_tis.inp_amount:]))
    for i, val := range text[1:] {
        if len(val) <= 2 {continue}
        if i<ret_tis.inp_amount {
            inp_node_string := strings.Split(val, ";")
            node_string := strings.Split(strings.Trim(inp_node_string[0]," "), " ")
            edge_temp := make([]edge, len(node_string[1:]))
            var values []int
            for _, v := range strings.Split(strings.Trim(inp_node_string[1]," "), " ") {
                temp, _ := strconv.Atoi(v)
                values = append(values, temp)
            }
            for j, v := range node_string[1:] {
                if v!="-1" && v!="0" {
                    temp, _ := strconv.Atoi(v)
                    edge_temp[j].dest = &ret_tis.nodes[temp-ret_tis.inp_amount] //reference to the node in tis: number of node minus inp und outp
                } else {
                    edge_temp[j].dest = nil
                }
            }
            ret_tis.inp_nodes[i] = inp_node{values,edge_temp,0,false}
        } else {
            node_string := strings.Split(val, " ")
            edge_temp := make([]edge, len(node_string[1:]))
            for j, v := range node_string[1:] {
                temp, _ := strconv.Atoi(v)
                if temp>=ret_tis.inp_amount {
                    edge_temp[j].dest = &ret_tis.nodes[temp-ret_tis.inp_amount] //reference to the node in tis: number of node minus inp
                } else {
                    edge_temp[j].dest = nil
                }
            }
            ret_tis.nodes[i-ret_tis.inp_amount] = node{source_to_code(code,ret_tis.loc),0,0,0,edge_temp,false,nil}
        }
    }
    return
}

func source_to_code(source string, loc int) []cmd { //TODO LABEL
    code := make([]cmd, loc)
    for i, val := range strings.Split(source,"\n") {
        if i>loc {
            break
        }
        cmd_string := strings.Split(val, " ")
        temp := cmd{cmd_codes[cmd_string[0]],0,0,0}
        if len(cmd_string) > 1 {
            if arg1, err := strconv.Atoi(cmd_string[1]); err == nil {
                temp.arg1 = arg1
            } else {
                temp.arg1 = reg_codes[cmd_string[1]]
                temp.mode |= 2
            }
        }
        if len(cmd_string) > 2 {
            if arg2, err := strconv.Atoi(cmd_string[2]); err == nil {
                temp.arg2 = arg2
            } else {
                temp.arg2 = reg_codes[cmd_string[2]]
                temp.mode |= 1
            }
        }
        code[i] = temp
    }
    return code
}

func main() {
    source, err := ioutil.ReadFile("test.tis");
    if err != nil {
        panic(err)
    }
    code, err := ioutil.ReadFile("test.code");
    if err != nil {
        panic(err)
    }
    tis200 := construct_tis(string(source),string(code))
    tis200.run()
}
