package main

import (
	"fmt"
	"raft/internal/comun/check"

	//"log"
	//"crypto/rand"

	"os"
	"strconv"
	"time"

	"raft/internal/comun/rpctimeout"
	"raft/internal/raft"
)

const (
	//nodos replicas
	REPLICA1 = "ss-0.ss-service.default.svc.cluster.local:6000"
	REPLICA2 = "ss-1.ss-service.default.svc.cluster.local:6000"
	REPLICA3 = "ss-2.ss-service.default.svc.cluster.local:6000"
)

// ---------------------------------------------------------------------
//
// Canal de resultados de ejecución de comandos ssh remotos
type canalResultados chan string

func (cr canalResultados) stop() {
	close(cr)

	// Leer las salidas obtenidos de los comandos ssh ejecutados
	for s := range cr {
		fmt.Println(s)
	}
}

// ---------------------------------------------------------------------
// Operativa en configuracion de despliegue y pruebas asociadas
type configDespliegue struct {
	conectados  []bool
	numReplicas int
	nodosRaft   []rpctimeout.HostPort
	cr          canalResultados
}

// Crear una configuracion de despliegue
func makeCfgDespliegue(n int, nodosraft []string,
	conectados []bool) *configDespliegue {
	cfg := &configDespliegue{}
	cfg.conectados = conectados
	cfg.numReplicas = n
	cfg.nodosRaft = rpctimeout.StringArrayToHostPortArray(nodosraft)
	cfg.cr = make(canalResultados, 2000)

	return cfg
}

func (cfg *configDespliegue) stop() {
	//cfg.stopDistributedProcesses()

	time.Sleep(50 * time.Millisecond)

	cfg.cr.stop()
}

//---------------------------TESTS---------------------------------------

// Se pone en marcha una replica ?? - 3 NODOS RAFT
func soloArranqueYparadaTest1(cfg *configDespliegue) {
	//t.Skip("SKIPPED soloArranqueYparadaTest1")

	fmt.Println(" test 1.....................")

	// Comprobar estado replica 0
	cfg.comprobarEstadoRemoto(0, 0, false, -1)

	// Comprobar estado replica 1
	cfg.comprobarEstadoRemoto(1, 0, false, -1)

	// Comprobar estado replica 2
	cfg.comprobarEstadoRemoto(2, 0, false, -1)

	fmt.Println(".............test 1 Superado")
}

// Primer lider en marcha - 3 NODOS RAFT
func elegirPrimerLiderTest2(cfg *configDespliegue) {
	//t.Skip("SKIPPED ElegirPrimerLiderTest2")

	fmt.Println("test2.....................")

	// Se ha elegido lider ?
	fmt.Printf("Probando lider en curso\n")
	cfg.pruebaUnLider(3)
	fmt.Println("Lider elegido: ", cfg.lider())

	fmt.Println(".............test2 Superado")
}

// Fallo de un primer lider y reeleccion de uno nuevo - 3 NODOS RAFT
func falloAnteriorElegirNuevoLiderTest3(cfg *configDespliegue) {
	//t.Skip("SKIPPED FalloAnteriorElegirNuevoLiderTest3")

	fmt.Println("test3 .....................")
	//cfg.pruebaUnLider(3)

	//dejo tiempo para que eligan lider
	time.Sleep(3000 * time.Millisecond)

	var reply raft.Vacio

	idLider := cfg.lider()

	fmt.Println("Lider inicial", idLider)
	endPoint := cfg.nodosRaft[idLider]
	//paramos ese nodo
	err := endPoint.CallTimeout("NodoRaft.ParaNodo",
		raft.Vacio{}, &reply, 10000*time.Millisecond)
	check.CheckError(err, "Error en llamada RPC Para nodo")

	cfg.conectados[idLider] = false
	fmt.Println("Lider desconectado")

	//dejo tiempo para que eligan lider
	time.Sleep(5000 * time.Millisecond)

	idLider = cfg.lider()

	fmt.Println("Lider nuevo", idLider)

	// Parar réplicas almacenamiento en remoto

	fmt.Println(".............test 3Superado")
}

// 3 operaciones comprometidas con situacion estable y sin fallos - 3 NODOS RAFT
func tresOperacionesComprometidasEstable(cfg *configDespliegue) {
	//t.Skip("SKIPPED tresOperacionesComprometidasEstable")

	fmt.Println("test 4.....................")

	// Se ha elegido lider ?
	fmt.Printf("Probando lider en curso\n")
	cfg.pruebaUnLider(3)

	//dejo tiempo para que eligan lider
	time.Sleep(5000 * time.Millisecond)

	idLider := cfg.lider()
	fmt.Println("Lider: ", idLider)

	endPoint := cfg.nodosRaft[idLider]
	var reply raft.Vacio
	//ejecutamos operaciones
	println("Sometiendo operacion de escritura")
	operacion1 := raft.TipoOperacion{Operacion: "escribir", Clave: "1", Valor: "prueba1"}
	err := endPoint.CallTimeout("NodoRaft.SometerOperacionRaft", operacion1, &reply, 500*time.Millisecond)
	check.CheckError(err, "Error en la operacion 1")
	println("Operacion de escritura sometida")
	time.Sleep(5000 * time.Millisecond)
	println("Sometiendo operacion de lectura")
	operacion2 := raft.TipoOperacion{Operacion: "leer", Clave: "1", Valor: ""}
	err = endPoint.CallTimeout("NodoRaft.SometerOperacionRaft", operacion2, &reply, 500*time.Millisecond)
	check.CheckError(err, "Error en la operacion 2")
	println("Operacion de escritura sometida")
	time.Sleep(5000 * time.Millisecond)
	println("Sometiendo operacion de escritura")
	operacion3 := raft.TipoOperacion{Operacion: "escribir", Clave: "2", Valor: "prueba2"}
	err = endPoint.CallTimeout("NodoRaft.SometerOperacionRaft", operacion3, &reply, 500*time.Millisecond)
	check.CheckError(err, "Error en la operacion 3")
	println("Operacion de escritura sometida")
	time.Sleep(5000 * time.Millisecond)

	// Parar réplicas alamcenamiento en remoto

	fmt.Println(".............test 4 Superado")
}

// Se consigue acuerdo a pesar de desconexiones de seguidor -- 3 NODOS RAFT
func AcuerdoApesarDeSeguidor(cfg *configDespliegue) {
	//t.Skip("SKIPPED AcuerdoApesarDeSeguidor")
	fmt.Println("test 5.....................")
	time.Sleep(1000 * time.Millisecond)
	// Comprometer una entrada
	fmt.Printf("Probando lider en curso\n")
	cfg.pruebaUnLider(3)

	//dejo tiempo para que eligan lider
	time.Sleep(3000 * time.Millisecond)

	//_, _, esLider, idLider := cfg.obtenerEstadoRemoto(1)
	idLider := cfg.lider()
	var reply raft.Vacio
	println("Lider elegido: ", idLider)

	time.Sleep(3000 * time.Millisecond)

	println("Sometiendo operacion")
	cfg.operacion(idLider, "escribir", "1", "entrada1")
	println("Operacion de escritura sometida")

	//  Obtener un lider y, a continuación desconectar una de los nodos Raft

	//dejo tiempo para que eligan lider
	time.Sleep(3000 * time.Millisecond)

	// Desconectar seguidor
	var nodo int
	idLider = cfg.lider()
	if idLider == 0 {
		nodo = 1
	} else {
		nodo = 0
	}

	println("Desconectando a", nodo)

	endPoint := cfg.nodosRaft[nodo]
	//paramos ese nodo
	err := endPoint.CallTimeout("NodoRaft.ParaNodo",
		raft.Vacio{}, &reply, 200*time.Millisecond)
	check.CheckError(err, "Error en llamada RPC Para nodo")

	cfg.conectados[nodo] = false
	// Comprobar varios acuerdos con una réplica desconectada
	//_, _, esLider, idLider = cfg.obtenerEstadoRemoto(1)
	time.Sleep(5000 * time.Millisecond)

	//Buscamos lider de nuevo
	idLider = cfg.lider()
	println("Lider elegido: ", idLider)
	//ejecutamos operaciones
	replyOp := cfg.operacion(idLider, "leer", "1", "")
	if replyOp != "entrada1" {
		fmt.Printf("Lectura incorrecta")
	} else {
		println("Operacion de lectura sometida correctamente")
	}
	cfg.operacion(idLider, "escribir", "2", "entrada2")
	replyOp = cfg.operacion(idLider, "leer", "2", "")
	if replyOp != "entrada2" {
		fmt.Printf("Lectura incorrecta")
	} else {
		println("Operacion de lectura sometida correctamente")
	}
	// reconectar nodo Raft previamente desconectado y comprobar varios acuerdos

	// Parar réplicas alamcenamiento en remoto

	//cfg.stopDistributedProcesses() // Parametros

	fmt.Println(".............test 5 Superado")

}

// NO se consigue acuerdo al desconectarse mayoría de seguidores -- 3 NODOS RAFT
func SinAcuerdoPorFallos(cfg *configDespliegue) {
	//t.Skip("SKIPPED SinAcuerdoPorFallos")
	fmt.Println("test 6.....................")
	time.Sleep(1000 * time.Millisecond)
	// Comprometer una entrada
	fmt.Printf("Probando lider en curso\n")

	// Comprometer una entrada

	cfg.pruebaUnLider(3)

	//dejo tiempo para que eligan lider
	time.Sleep(1000 * time.Millisecond)

	idLider := cfg.lider()
	fmt.Println("Lider inicial: ", idLider)
	var reply raft.Vacio

	time.Sleep(1000 * time.Millisecond)

	println("Sometiendo operacion")
	cfg.operacion(idLider, "escribir", "1", "entrada1")
	println("Operacion de escritura sometida")

	//  Obtener un lider y, a continuación desconectar una de los nodos Raft
	//cfg.pruebaUnLider(3)

	//dejo tiempo para que eligan lider
	time.Sleep(3000 * time.Millisecond)

	// Desconectar seguidor
	for i := range cfg.nodosRaft {
		if i != idLider {
			endPoint := cfg.nodosRaft[i]
			//paramos ese nodo
			err := endPoint.CallTimeout("NodoRaft.ParaNodo",
				raft.Vacio{}, &reply, 10000*time.Millisecond)
			check.CheckError(err, "Error en llamada RPC Para nodo")

			cfg.conectados[i] = false
			fmt.Println("Desconcectando nodo", i)
		}

	}
	// Comprobar varios acuerdos con una réplica desconectada

	//Buscamos lider de nuevo
	idLider = cfg.lider()
	fmt.Println("Lider nuevo: ", idLider)
	//ejecutamos operaciones
	replyOp := cfg.operacion(idLider, "leer", "1", "")
	if replyOp != "entrada1" {
		fmt.Println("Lectura incorrecta")
	} else {
		fmt.Println("Lectura correcta")
	}
	time.Sleep(1000 * time.Millisecond)
	println("Intentando someter operacion de escritura")
	cfg.operacion(idLider, "escribir", "2", "entrada2")
	replyOp = cfg.operacion(idLider, "leer", "2", "")
	if replyOp != "entrada2" {
		fmt.Println("Lectura incorrecta")
	} else {
		fmt.Println("Lectura correcta")
	}

	// Parar réplicas alamcenamiento en remoto
	//cfg.stopDistributedProcesses() // Parametros

	fmt.Println(".............test 6 Superado")
}

// Se somete 5 operaciones de forma concurrente -- 3 NODOS RAFT
func SometerConcurrentementeOperaciones(cfg *configDespliegue) {

	//t.Skip("SKIPPED SometerConcurrentementeOperaciones")

	fmt.Println("test 7.....................")
	time.Sleep(1000 * time.Millisecond)
	// Comprometer una entrada
	fmt.Printf("Probando lider en curso\n")

	cfg.pruebaUnLider(3)

	// un bucle para estabilizar la ejecucion
	time.Sleep(3000 * time.Millisecond)

	// Obtener un lider y, a continuación someter una operacion
	idLider := cfg.lider()
	cfg.operacion(idLider, "escribir", "1", "entrada 1")
	println("Entrada de escritura sometida")
	replyOp := cfg.operacion(idLider, "leer", "1", "")
	if replyOp != "entrada 1" {
		fmt.Println("Lectura incorrecta")
	} else {
		fmt.Println("Lectura correcta")
	}
	time.Sleep(5000 * time.Millisecond)
	// Someter 5  operaciones concurrentes
	for i := 1; i <= 5; i++ {
		go func(i int) {
			cfg.operacion(idLider, "escribir", strconv.Itoa(i+1),
				"entrada "+strconv.Itoa(i+1))
		}(i)
	}
	time.Sleep(5000 * time.Millisecond)
	replyOp = cfg.operacion(idLider, "leer", "2", "")
	println("Se ha leido el valor: ", replyOp)
	time.Sleep(3000 * time.Millisecond)
	replyOp = cfg.operacion(idLider, "leer", "3", "")
	println("Se ha leido el valor: ", replyOp)
	time.Sleep(3000 * time.Millisecond)
	replyOp = cfg.operacion(idLider, "leer", "4", "")
	println("Se ha leido el valor: ", replyOp)
	time.Sleep(3000 * time.Millisecond)
	replyOp = cfg.operacion(idLider, "leer", "5", "")
	println("Se ha leido el valor: ", replyOp)
	time.Sleep(3000 * time.Millisecond)
	replyOp = cfg.operacion(idLider, "leer", "6", "")
	println("Se ha leido el valor: ", replyOp)
	time.Sleep(3000 * time.Millisecond)
	// Comprobar estados de nodos Raft, sobre todo
	// el avance del mandato en curso e indice de registro de cada uno
	// que debe ser identico entre ellos
	_, mandato0, _, _ := cfg.obtenerEstadoRemoto(0)
	_, mandato1, _, _ := cfg.obtenerEstadoRemoto(1)
	_, mandato2, _, _ := cfg.obtenerEstadoRemoto(2)
	if (mandato0 == mandato1) && (mandato1 == mandato2) {
		fmt.Println("Los mandatos coinciden")
	} else {
		fmt.Println("Los mandatos no coinciden")
	}
	if cfg.compararlogs() {
		fmt.Println("Los índices coinciden")
	} else {
		fmt.Println("Los índices no coinciden")
	}

	// Parar réplicas alamcenamiento en remoto
	//cfg.stopDistributedProcesses() // Parametros

	fmt.Println(".............test 7 Superado")
}

// --------------------------------------------------------------------------
// FUNCIONES DE SUBTESTS
// FUNCIONES DE APOYO
// Comprobar que hay un solo lider
// probar varias veces si se necesitan reelecciones
func (cfg *configDespliegue) pruebaUnLider(numreplicas int) int {
	for iters := 0; iters < 10; iters++ {
		time.Sleep(1000 * time.Millisecond)
		mapaLideres := make(map[int][]int)
		for i := 0; i < numreplicas; i++ {
			if cfg.conectados[i] {
				if _, mandato, eslider, _ := cfg.obtenerEstadoRemoto(i); eslider {
					mapaLideres[mandato] = append(mapaLideres[mandato], i)
				}
			}
		}

		ultimoMandatoConLider := -1
		for mandato, lideres := range mapaLideres {
			if len(lideres) > 1 {
				fmt.Println("mandato %d tiene %d (>1) lideres",
					mandato, len(lideres))
			}
			if mandato > ultimoMandatoConLider {
				ultimoMandatoConLider = mandato
			}
		}

		if len(mapaLideres) != 0 {

			return mapaLideres[ultimoMandatoConLider][0] // Termina

		}
	}
	fmt.Println("un lider esperado, ninguno obtenido")

	return -1 // Termina
}
func (cfg *configDespliegue) obtenerEstadoRemoto(
	indiceNodo int) (int, int, bool, int) {
	var reply raft.EstadoRemoto
	err := cfg.nodosRaft[indiceNodo].CallTimeout("NodoRaft.ObtenerEstadoNodo",
		raft.Vacio{}, &reply, 10000*time.Millisecond)
	check.CheckError(err, "Error en llamada RPC ObtenerEstadoRemoto")

	return reply.IdNodo, reply.Mandato, reply.EsLider, reply.IdLider
}

// Comprobar estado remoto de un nodo con respecto a un estado prefijado
func (cfg *configDespliegue) comprobarEstadoRemoto(idNodoDeseado int,
	mandatoDeseado int, esLiderDeseado bool, IdLiderDeseado int) {
	idNodo, mandato, esLider, idLider := cfg.obtenerEstadoRemoto(idNodoDeseado)

	fmt.Println("Estado replica: ", idNodo, mandato, esLider, idLider, "\n")

	/*if idNodo != idNodoDeseado || mandato != mandatoDeseado ||
		esLider != esLiderDeseado || idLider != IdLiderDeseado {
		fmt.Println("Estado incorrecto en replica %d",
			idNodoDeseado)
	}*/
	if idNodo != idNodoDeseado {
		fmt.Println("Estado incorrecto en replica %d",
			idNodoDeseado)
	}

}

func (cfg *configDespliegue) lider() int {
	for i := range cfg.nodosRaft {
		if cfg.conectados[i] {
			_, _, esLider, _ := cfg.obtenerEstadoRemoto(i)
			if esLider {
				return i
			}
		}
	}
	return -1
}

func (cfg *configDespliegue) stopDistributedProcesses() {
	var reply raft.Vacio

	for _, endPoint := range cfg.nodosRaft {
		err := endPoint.CallTimeout("NodoRaft.ParaNodo",
			raft.Vacio{}, &reply, 10000*time.Millisecond)
		check.CheckError(err, "Error en llamada RPC Para nodo")
	}
}

func Abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func (cfg *configDespliegue) operacion(idLider int, op string,
	clave string, valor string) string {
	tipo := raft.TipoOperacion{Operacion: op, Clave: clave, Valor: valor}
	var reply raft.ResultadoRemoto
	endPoint := cfg.nodosRaft[idLider]
	err := endPoint.CallTimeout("NodoRaft.SometerOperacionRaft",
		tipo, &reply, 10000*time.Millisecond)
	check.CheckError(err, "Error en la operacion")
	time.Sleep(1000 * time.Millisecond)
	return reply.ValorADevolver
}

func (cfg *configDespliegue) compararlogs() bool {
	var reply0 raft.LogRemoto
	endPoint := cfg.nodosRaft[0]
	err := endPoint.CallTimeout("NodoRaft.ObtenerLog",
		raft.Vacio{}, &reply0, 10000*time.Millisecond)
	check.CheckError(err, "Error en llamada RPC Obtener log nodo")
	var reply1 raft.LogRemoto
	endPoint = cfg.nodosRaft[1]
	err = endPoint.CallTimeout("NodoRaft.ObtenerLog",
		raft.Vacio{}, &reply1, 10000*time.Millisecond)
	check.CheckError(err, "Error en llamada RPC Obtener log nodo")
	var reply2 raft.LogRemoto
	endPoint = cfg.nodosRaft[2]
	err = endPoint.CallTimeout("NodoRaft.ObtenerLog",
		raft.Vacio{}, &reply2, 10000*time.Millisecond)
	check.CheckError(err, "Error en llamada RPC Obtener log nodo")

	if (reply0.Log.Index != reply1.Log.Index) || (reply2.Log.Index != reply1.Log.Index) ||
		reply0.Log.Index != reply2.Log.Index {
		return false
	} else {
		return true
	}
}

//-----------------MAIN------------------------------

func main() {
	time.Sleep(5000 * time.Millisecond)
	cfg := makeCfgDespliegue(3,
		[]string{REPLICA1, REPLICA2, REPLICA3},
		[]bool{true, true, true})
	switch os.Args[1] {
	case "1":
		soloArranqueYparadaTest1(cfg)
	case "2":
		elegirPrimerLiderTest2(cfg)
	case "3":
		falloAnteriorElegirNuevoLiderTest3(cfg)
	case "4":
		tresOperacionesComprometidasEstable(cfg)
	case "5":
		AcuerdoApesarDeSeguidor(cfg)
	case "6":
		time.Sleep(3000 * time.Millisecond)
		SinAcuerdoPorFallos(cfg)
	case "7":
		time.Sleep(3000 * time.Millisecond)
		SometerConcurrentementeOperaciones(cfg)
	}
}
