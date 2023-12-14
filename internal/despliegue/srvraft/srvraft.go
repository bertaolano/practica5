package main

import (
	//"errors"
	"fmt"
	"strings"

	//"log"
	"net"
	"net/rpc"
	"os"
	"raft/internal/comun/check"
	"raft/internal/comun/rpctimeout"
	"raft/internal/raft"
	"strconv"
	//"time"
)

func main() {
	DNS := "ss-service.default.svc.cluster.local:6000"

	// obtener entero de indice de este nodo
	yo := os.Args[1]
	nombre := strings.Split(yo, "-")[0]
	me, err := strconv.Atoi(strings.Split(yo, "-")[1])
	check.CheckError(err, "Main, mal numero entero de indice de nodo:")

	var dir []string
	for i := 0; i < 3; i++ {
		nodo := nombre + "-" + strconv.Itoa(i) + "." + DNS
		dir = append(dir, nodo)
	}

	var nodos []rpctimeout.HostPort
	// Resto de argumento son los end points como strings
	// De todas la replicas-> pasarlos a HostPort
	for _, endPoint := range os.Args[2:] {
		nodos = append(nodos, rpctimeout.HostPort(endPoint))
	}

	// Parte Servidor
	nr := raft.NuevoNodo(nodos, me, make(chan raft.AplicaOperacion, 1000))
	rpc.Register(nr)

	fmt.Println("Replica escucha en :", me, " de ", os.Args[2:])

	l, err := net.Listen("tcp", os.Args[2:][me])
	check.CheckError(err, "Main listen error:")

	rpc.Accept(l)
}
