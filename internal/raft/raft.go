// Escribir vuestro código de funcionalidad Raft en este fichero
//

package raft

//
// API
// ===
// Este es el API que vuestra implementación debe exportar
//
// nodoRaft = NuevoNodo(...)
//   Crear un nuevo servidor del grupo de elección.
//
// nodoRaft.Para()
//   Solicitar la parado de un servidor
//
// nodo.ObtenerEstado() (yo, mandato, esLider)
//   Solicitar a un nodo de elección por "yo", su mandato en curso,
//   y si piensa que es el msmo el lider
//
// nodoRaft.SometerOperacion(operacion interface()) (indice, mandato, esLider)

// type AplicaOperacion

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"

	//"crypto/rand"
	"sync"
	"time"

	//"net/rpc"

	"raft/internal/comun/rpctimeout"
)

const (
	// Constante para fijar valor entero no inicializado
	IntNOINICIALIZADO = -1

	//  false deshabilita por completo los logs de depuracion
	// Aseguraros de poner kEnableDebugLogs a false antes de la entrega
	kEnableDebugLogs = false

	// Poner a true para logear a stdout en lugar de a fichero
	kLogToStdout = false

	// Cambiar esto para salida de logs en un directorio diferente
	kLogOutputDir = "./logs_raft/"

	heartbeatTime = 50 * time.Millisecond
)

type TipoOperacion struct {
	Operacion string // La operaciones posibles son "leer" y "escribir"
	Clave     string
	Valor     string // en el caso de la lectura Valor = ""
}

// A medida que el nodo Raft conoce las operaciones de las  entradas de registro
// comprometidas, envía un AplicaOperacion, con cada una de ellas, al canal
// "canalAplicar" (funcion NuevoNodo) de la maquina de estados
type AplicaOperacion struct {
	Indice    int // en la entrada de registro
	Operacion TipoOperacion
}

type Log struct {
	Index int
	Term  int
	Op    TipoOperacion
}

// Tipo de dato Go que representa un solo nodo (réplica) de raft
type NodoRaft struct {
	Mux sync.Mutex // Mutex para proteger acceso a estado compartido

	// Host:Port de todos los nodos (réplicas) Raft, en mismo orden
	Nodos   []rpctimeout.HostPort
	Yo      int // indice de este nodo en campo array "nodos"
	IdLider int
	// Utilización opcional de este logger para depuración
	// Cada nodo Raft tiene su propio registro de trazas (logs)
	Logger *log.Logger

	//0 -> seguidor, 1 -> candidato, 2 -> líder
	Estado int

	// Vuestros datos aqui.
	CurrentTerm int   //último mandato visto por el servidor
	VotedFor    int   //id del candidato
	Log         []Log //comandos para la máquina de estados	?????
	CommitIndex int   //índice de la mayor entrada log que se va a someter
	LastApplied int   //índice más alto de la entrada log aplicado a la
	//máquina de estados
	NextIndex []int //para cada servidor, índice de la siguiente entrada
	//log que va a enviar a ese servidor
	MatchIndex []int //para cada servidor, índice de la siguiente entrada
	//log que se va a replicar en el servidor
	VotesRec           int       //número de votos recibidos
	chanLatidos        chan bool //recibe latido
	ChanAplicaOp       chan AplicaOperacion
	ChanCommitLog      chan string //entrada consolidada
	NumRespuestas      int         //número de respuestas recibidas
	NumNodosOpSometida int
	MaquinaEstados     map[string]string
}

// Creacion de un nuevo nodo de eleccion
//
// Tabla de <Direccion IP:puerto> de cada nodo incluido a si mismo.
//
// <Direccion IP:puerto> de este nodo esta en nodos[yo]
//
// Todos los arrays nodos[] de los nodos tienen el mismo orden

// canalAplicar es un canal donde, en la practica 5, se recogerán las
// operaciones a aplicar a la máquina de estados. Se puede asumir que
// este canal se consumira de forma continúa.
//
// NuevoNodo() debe devolver resultado rápido, por lo que se deberían
// poner en marcha Gorutinas para trabajos de larga duracion
func NuevoNodo(nodos []rpctimeout.HostPort, yo int,
	canalAplicarOperacion chan AplicaOperacion) *NodoRaft {
	nr := &NodoRaft{}
	nr.Nodos = nodos
	nr.Yo = yo
	nr.IdLider = -1

	if kEnableDebugLogs {
		nombreNodo := nodos[yo].Host() + "_" + nodos[yo].Port()
		logPrefix := fmt.Sprintf("%s", nombreNodo)

		fmt.Println("LogPrefix: ", logPrefix)

		if kLogToStdout {
			nr.Logger = log.New(os.Stdout, nombreNodo+" -->> ",
				log.Lmicroseconds|log.Lshortfile)
		} else {
			err := os.MkdirAll(kLogOutputDir, os.ModePerm)
			if err != nil {
				panic(err.Error())
			}
			logOutputFile, err := os.OpenFile(fmt.Sprintf("%s/%s.txt",
				kLogOutputDir, logPrefix), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
			if err != nil {
				panic(err.Error())
			}
			nr.Logger = log.New(logOutputFile,
				logPrefix+" -> ", log.Lmicroseconds|log.Lshortfile)
		}
		nr.Logger.Println("logger initialized")
	} else {
		nr.Logger = log.New(ioutil.Discard, "", 0)
	}

	// código de inicialización
	nr.Estado = 0
	nr.CurrentTerm = 0
	nr.VotedFor = -1
	//nr.Log = make([]Log, len(nr.Nodos)+1)
	nr.CommitIndex = 0
	nr.LastApplied = 0
	nr.NextIndex = make([]int, len(nr.Nodos)+1)
	nr.NextIndex = append(nr.NextIndex, len(nr.Log)+1)
	nr.MatchIndex = make([]int, len(nr.Nodos)+1)
	nr.MatchIndex = append(nr.MatchIndex, 0)
	nr.VotesRec = 0
	nr.chanLatidos = make(chan bool)
	nr.ChanAplicaOp = canalAplicarOperacion
	nr.ChanCommitLog = make(chan string)
	nr.NumRespuestas = 0
	nr.NumNodosOpSometida = 0
	nr.MaquinaEstados = make(map[string]string)

	go nr.gestionLider()

	return nr
}

func (nr *NodoRaft) ejecutarOperacion(operacion Log) (string, bool) {
	switch operacion.Op.Operacion {
	case "leer":
		val := nr.MaquinaEstados[operacion.Op.Clave]
		println("Valor leido: ", operacion.Op.Clave, " ", val)
		return val, val != ""
	case "escribir":
		nr.MaquinaEstados[operacion.Op.Clave] = operacion.Op.Valor
		println("Valor escrito: ", operacion.Op.Clave, " ",
			nr.MaquinaEstados[operacion.Op.Clave])
		return "", true
	}
	return "", false
}

// funcion encargada de enviar latidos al resto si es lider y recibir en otro caso
func (nr *NodoRaft) gestionLider() {
	nr.Logger.Println(nr.Estado)
	//time.Sleep(10000 * time.Millisecond)
	for {
		switch nr.Estado {
		case 2: //es lider
			nr.Logger.Println("Soy lider")
			nr.Mux.Lock()
			nr.IdLider = nr.Yo
			nr.Mux.Unlock()
			for i := 0; i <= len(nr.Nodos)-1; i++ {
				if i != nr.Yo {
					//enviamos latidos a los nodos
					nr.Logger.Println("Enviando latido a ", i)
					mayoria := nr.enviar(i)
					//Hay consenso, se puede someter la nueva entrada
					if mayoria && nr.LastApplied < len(nr.Log)-1 {
						nr.ejecutarOperacion(nr.Log[nr.LastApplied])
						nr.LastApplied++
						println("nueva entrada comprometida")
					}
				}
			}
			time.Sleep(heartbeatTime)
		case 0, 1: // seguidor
			nr.recibirLatido()
		}
	}
}

func (nr *NodoRaft) enviarLatidoA(i int, args *ArgAppendEntries, res *Results) bool {
	err := nr.Nodos[i].CallTimeout("NodoRaft.AppendEntries", args, res, 200*time.Millisecond)
	if err == nil {
		if res.Term > nr.CurrentTerm {
			//he enviado latido a nodo con mayor mandato => actualizo mi mandato y vuelvo a ser seguidor
			nr.Mux.Lock()
			nr.CurrentTerm = res.Term
			nr.IdLider = -1
			nr.Estado = 0
			nr.Mux.Unlock()
		}
		return true
	} else {
		return false
	}
}

func (nr *NodoRaft) enviarEntrada(i int, args *ArgAppendEntries, res *Results) bool {
	err := nr.Nodos[i].CallTimeout("NodoRaft.AppendEntries",
		args, res, 200*time.Millisecond)
	if err == nil {
		if res.Success {
			//si log consistente => actualiza MatchIndex y NextIndex
			nr.MatchIndex[i] = nr.NextIndex[i]
			nr.NextIndex[i]++
			nr.Mux.Lock()
			if nr.MatchIndex[i] > nr.CommitIndex {
				//servidor replica entrada aún no cometida,
				//compruebo si tengo mayoria de respuestas
				nr.NumRespuestas++
				if nr.NumRespuestas > len(nr.Nodos)/2 {
					//me han contestado la mayoría de nodos => consolida entrada
					nr.CommitIndex++
					nr.NumRespuestas = 0
				}
			}
			nr.Mux.Unlock()
		} else {
			//log de seguidor inconsistente
			nr.NextIndex[i]--
		}
		return true
	} else {
		return false
	}
}

func (nr *NodoRaft) enviar(i int) bool {
	var res Results
	mayoria := false

	if len(nr.Log)-1 >= nr.NextIndex[i] {
		//hay nuevas entradas en el log => las envía
		log := Log{nr.NextIndex[i], nr.Log[nr.NextIndex[i]].Term,
			nr.Log[nr.NextIndex[i]].Op}
		if nr.NextIndex[i] != 0 {
			args := ArgAppendEntries{nr.CurrentTerm, nr.Yo, nr.NextIndex[i] - 1,
				nr.Log[nr.NextIndex[i]-1].Term, log, nr.CommitIndex}
			mayoria = nr.enviarEntrada(i, &args, &res)
		} else {
			args := ArgAppendEntries{nr.CurrentTerm, nr.Yo, 0,
				0, log, nr.CommitIndex}
			mayoria = nr.enviarEntrada(i, &args, &res)
		}
	} else { // si no hay envía latido
		if nr.NextIndex[i] != 0 {
			args := ArgAppendEntries{nr.CurrentTerm, nr.Yo, nr.NextIndex[i] - 1,
				nr.Log[nr.NextIndex[i]-1].Term, Log{}, nr.CommitIndex}
			mayoria = nr.enviarLatidoA(i, &args, &res)
		} else {
			args := ArgAppendEntries{nr.CurrentTerm, nr.Yo, 0,
				0, Log{}, nr.CommitIndex}
			mayoria = nr.enviarLatidoA(i, &args, &res)
		}
	}
	return mayoria
}

func (nr *NodoRaft) recibirLatido() {
	nr.Logger.Println("Esperando a recibir latido")
	select {
	case <-nr.chanLatidos: //hay lider
		nr.Logger.Println("Me ha llegado un latido")
	/*case <-nr.ChanAplicaOp: //hay lider
	nr.Logger.Println("Me ha llegado una entrada")*/
	case <-time.After((time.Duration(rand.Intn(150) + 150)) * time.Millisecond):
		//pasa tiempo de expiración sin recibir mensaje (entre 100 y 400ms)
		nr.Logger.Println("Ha acabado mi timeOut")
		nr.Mux.Lock()
		if nr.Estado == 0 { // si soy seguidor paso a candidato
			nr.Estado = 1
			nr.Logger.Println("Me convierto en candidato")
		}
		nr.Mux.Unlock()
		// inicia elección de líder
		nr.inicioEleccion()
	}
}

// Metodo Para() utilizado cuando no se necesita mas al nodo
//
// Quizas interesante desactivar la salida de depuracion
// de este nodo
func (nr *NodoRaft) para() {
	go func() { time.Sleep(25 * time.Millisecond); os.Exit(0) }()
}

// Devuelve "yo", mandato en curso y si este nodo cree ser lider
//
// Primer valor devuelto es el indice de este  nodo Raft el el conjunto de nodos
// la operacion si consigue comprometerse.
// El segundo valor es el mandato en curso
// El tercer valor es true si el nodo cree ser el lider
// Cuarto valor es el lider, es el indice del líder si no es él
func (nr *NodoRaft) obtenerEstado() (int, int, bool, int) {
	var yo int
	var mandato int
	var esLider bool
	var idLider int

	// Vuestro codigo
	//nr.Mux.Lock()
	yo = nr.Yo
	mandato = nr.CurrentTerm
	esLider = (nr.Estado == 2)
	idLider = nr.IdLider
	//nr.Mux.Unlock()
	return yo, mandato, esLider, idLider
}

// Devuelve la última entrada del log
func (nr *NodoRaft) obtenerLog() Log {
	return nr.Log[len(nr.Log)-1]
}

// El servicio que utilice Raft (base de datos clave/valor, por ejemplo)
// Quiere buscar un acuerdo de posicion en registro para siguiente operacion
// solicitada por cliente.

// Si el nodo no es el lider, devolver falso
// Sino, comenzar la operacion de consenso sobre la operacion y devolver en
// cuanto se consiga
//
// No hay garantia que esta operacion consiga comprometerse en una entrada de
// de registro, dado que el lider puede fallar y la entrada ser reemplazada
// en el futuro.
// Primer valor devuelto es el indice del registro donde se va a colocar
// la operacion si consigue comprometerse.
// El segundo valor es el mandato en curso
// El tercer valor es true si el nodo cree ser el lider
// Cuarto valor es el lider, es el indice del líder si no es él
func (nr *NodoRaft) someterOperacion(operacion TipoOperacion) (int, int,
	bool, int, string) {
	nr.Mux.Lock()
	indice := -1
	mandato := -1
	EsLider := (nr.Estado == 2)
	idLider := -1
	valorADevolver := ""

	// Vuestro codigo aqui
	if EsLider {
		println("Someter operacion: Soy lider")
		nr.Logger.Println("soy lider")

		indice = len(nr.Log)
		mandato = nr.CurrentTerm
		idLider = nr.Yo
		nr.Logger.Println("append")
		//añado una nueva operacion en el log
		nr.Log = append(nr.Log, Log{indice, mandato, operacion})
		println("añadida una nueva entrada con indice: ", indice,
			"y operacion ", operacion.Clave, operacion.Valor)
		nr.Mux.Unlock()
		time.Sleep(100 * time.Millisecond)
		if operacion.Operacion == "leer" {
			//leo solo operaciones ya comprometidas
			valorADevolver = nr.MaquinaEstados[operacion.Clave]
		}
	} else {
		nr.Logger.Println("No es lider")
		nr.Mux.Unlock()
		idLider = nr.IdLider
	}
	println("Voy a devolver el valor ", valorADevolver)

	return indice, mandato, EsLider, idLider, valorADevolver
}

// -----------------------------------------------------------------------
// LLAMADAS RPC al API
//
// Si no tenemos argumentos o respuesta estructura vacia (tamaño cero)
type Vacio struct{}

func (nr *NodoRaft) ParaNodo(args Vacio, reply *Vacio) error {
	defer nr.para()
	return nil
}

type EstadoParcial struct {
	Mandato int
	EsLider bool
	IdLider int
}

type EstadoRemoto struct {
	IdNodo int
	EstadoParcial
}

func (nr *NodoRaft) ObtenerEstadoNodo(args Vacio, reply *EstadoRemoto) error {
	reply.IdNodo, reply.Mandato, reply.EsLider, reply.IdLider = nr.obtenerEstado()
	return nil
}

type ResultadoRemoto struct {
	ValorADevolver string
	IndiceRegistro int
	EstadoParcial
}

func (nr *NodoRaft) SometerOperacionRaft(operacion TipoOperacion,
	reply *ResultadoRemoto) error {
	reply.IndiceRegistro, reply.Mandato, reply.EsLider,
		reply.IdLider, reply.ValorADevolver = nr.someterOperacion(operacion)
	return nil
}

type LogRemoto struct {
	Log Log
}

func (nr *NodoRaft) ObtenerLog(args Vacio, reply *LogRemoto) error {
	nr.Mux.Lock()
	reply.Log = nr.obtenerLog()
	nr.Mux.Unlock()
	return nil
}

// -----------------------------------------------------------------------
// LLAMADAS RPC protocolo RAFT
//
// Structura de ejemplo de argumentos de RPC PedirVoto.
//
// Recordar
// -----------
// Nombres de campos deben comenzar con letra mayuscula !
type ArgsPeticionVoto struct {
	// Vuestros datos aqui
	Term         int //mandato del candidato
	CandidateId  int //candidato que pide el voto
	LastLogIndex int //índice de la última entrada del log del candidato
	LastLogTerm  int //mandato de la última entrada del log del candidato
}

// Structura de ejemplo de respuesta de RPC PedirVoto,
//
// Recordar
// -----------
// Nombres de campos deben comenzar con letra mayuscula !
type RespuestaPeticionVoto struct {
	// Vuestros datos aqui
	Term        int  //mendato actual
	VoteGranted bool //el candidato ha recibido el voto
}

func (nr *NodoRaft) inicioEleccion() {
	nr.Logger.Println("Inicio eleccion")
	//me voto a mi mismo
	nr.Mux.Lock()
	nr.VotesRec = 1
	nr.VotedFor = nr.Yo
	nr.CurrentTerm++
	nr.Mux.Unlock()
	for i := 0; i <= len(nr.Nodos)-1; i++ {
		if i != nr.Yo {
			//envía petición de voto a todos menos a sí mismo
			nr.Logger.Println("Enviando peticion de voto a ", i)
			if len(nr.Log) != 0 {
				go nr.enviarPeticionVoto(i, &ArgsPeticionVoto{nr.CurrentTerm, nr.Yo,
					len(nr.Log) - 1, nr.Log[len(nr.Log)-1].Term}, &RespuestaPeticionVoto{})

			} else {
				go nr.enviarPeticionVoto(i, &ArgsPeticionVoto{nr.CurrentTerm, nr.Yo,
					-1, 0}, &RespuestaPeticionVoto{})
			}
		}
	}
}

// Funcion que devuelve true si el líder es el mejor podible
// Es decir, si el último mandato e ínidice es el mayor
func liderMejor(nr *NodoRaft, lastLogTerm int, lastLogIndex int) bool {
	if lastLogTerm > nr.Log[len(nr.Log)-1].Term {
		return true
	} else if lastLogTerm == nr.Log[len(nr.Log)-1].Term {
		if lastLogIndex >= len(nr.Log)-1 {
			return true
		} else {
			return false
		}
	} else {
		return false
	}
}

// Metodo para RPC PedirVoto
func (nr *NodoRaft) PedirVoto(peticion *ArgsPeticionVoto,
	reply *RespuestaPeticionVoto) error {
	// Vuestro codigo aqui
	if peticion.Term < nr.CurrentTerm {
		//petición de menor mandato => no te voto
		reply.Term = nr.CurrentTerm
		nr.Logger.Println("Recibido una paeticion de voto con menor mandato")
		reply.VoteGranted = false
	} else if peticion.Term == nr.CurrentTerm && peticion.CandidateId != nr.VotedFor {
		//recibo petición pero ya he votado en este mandato => no te voto
		reply.Term = nr.CurrentTerm
		reply.VoteGranted = false
	} else if peticion.Term > nr.CurrentTerm {
		//petición de mayor mandato => doy voto, actualizo mandato y si yo era
		//condidato o líder vuelvo a seguidor
		if len(nr.Log) == 0 || liderMejor(nr, peticion.LastLogIndex, peticion.LastLogTerm) {
			nr.Mux.Lock()
			nr.CurrentTerm = peticion.Term
			nr.VotedFor = peticion.CandidateId
			nr.Mux.Unlock()
			reply.Term = nr.CurrentTerm
			reply.VoteGranted = true
			nr.Logger.Println("He votado a ", peticion.CandidateId)
		} else {
			nr.Mux.Lock()
			nr.CurrentTerm = peticion.Term
			nr.Mux.Unlock()
			reply.Term = nr.CurrentTerm
			reply.VoteGranted = false
		}

		if nr.Estado == 1 || nr.Estado == 2 {
			nr.Logger.Println("Seguidor porque me llega peticion de mayor mandato")
			nr.Estado = 0
		}
	}

	return nil
}

type ArgAppendEntries struct {
	// Vuestros datos aqui
	Term         int //mandato del líder
	LeaderId     int //índice del líder
	PrevLogIndex int //índice de la entrada del log anterior a las nuevas
	PrevLogTerm  int //mandato de la entrada de PrevLogIndex
	Entries      Log //entrada que guardar en log (vacío si es latido)
	LeaderCommit int //último índice comprometido del líder
}

type Results struct {
	// Vuestros datos aqui
	Term    int  //mandato actual
	Success bool //si el log del seguidor tiene PrevLogIndex y PrevLogTerm, true
}

// Si el log no tiene entrada en PrevLogIndex con mandato igual a PrevLogTerm
// o si una entrada entra en conflicto con otra nueva con mismo índice pero
// tienen diferentes mandatos =>  devuelve falso
func logConsistent(nr *NodoRaft, PrevLogIndex int, PrevLogTerm int) bool {
	if PrevLogIndex > len(nr.Log)-1 || nr.Log[PrevLogIndex].Term != PrevLogTerm {
		return false
	} else {
		return true
	}
}

func min(a int, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}

// Metodo de tratamiento de llamadas RPC AppendEntries
func (nr *NodoRaft) AppendEntries(args *ArgAppendEntries,
	results *Results) error {
	// Completar....
	nr.Mux.Lock()
	if args.Term < nr.CurrentTerm {
		//recibo latido de nodo con menor mandato => no puede ser líder
		results.Term = nr.CurrentTerm
		results.Success = false
	} else if args.Term == nr.CurrentTerm {
		//recibo latido de líder con mismo mandato => lo reconozco como líder
		nr.IdLider = args.LeaderId
		results.Term = nr.CurrentTerm
		if len(nr.Log) == 0 {
			if args.Entries != (Log{}) {
				// recibo entrada nueva => la añado a mi log
				nr.Log = append(nr.Log, args.Entries)
			}
			results.Success = true
		} else if !logConsistent(nr, args.PrevLogIndex, args.PrevLogTerm) {
			//el log es inconsistente
			// rechazo entradas nuevas
			results.Success = false
		} else {
			// en mi log hay entrada PrevLogIndex con mandato PrevLogTerm
			if args.Entries != (Log{}) {
				//recibo nueva entrada =>
				//elimino mi log a partir de PrevLogIndex y añado entrada
				nr.Log = nr.Log[0 : args.PrevLogIndex+1]
				nr.Log = append(nr.Log, args.Entries)
			}
			results.Success = true
			nr.Logger.Println("Añadida nueva entrada en el log")
		}
		if args.LeaderCommit > nr.CommitIndex {
			//líder compromete nueva entrada => actualizar CommitIndex
			nr.CommitIndex = min(args.LeaderCommit, len(nr.Log)-1)
			nr.ejecutarOperacion(nr.Log[nr.LastApplied])
		}
		nr.chanLatidos <- true
	} else {
		//recibo latido de líder con mayor mandato => lo reconozco como líder
		nr.IdLider = args.LeaderId
		nr.CurrentTerm = args.Term
		results.Term = nr.CurrentTerm
		if nr.Estado == 2 {
			//si era líder vuelvo a ser seguidor
			nr.Logger.Println("Vuelvo seguidor porque recibo latido con mayor mandato")
			nr.Estado = 0
		} else {
			//si era candidato o seguidor recibe latido del nuevo líder
			if args.LeaderCommit > nr.CommitIndex {
				//líder compromete nueva entrada => actualizar CommitIndex
				nr.CommitIndex = min(args.LeaderCommit, len(nr.Log)-1)
			}
			nr.chanLatidos <- true
		}
	}
	nr.Mux.Unlock()
	return nil
}

// ----- Metodos/Funciones a utilizar como clientes
//
//

// Ejemplo de código enviarPeticionVoto
//
// nodo int -- indice del servidor destino en nr.nodos[]
//
// args *RequestVoteArgs -- argumentos par la llamada RPC
//
// reply *RequestVoteReply -- respuesta RPC
//
// Los tipos de argumentos y respuesta pasados a CallTimeout deben ser
// los mismos que los argumentos declarados en el metodo de tratamiento
// de la llamada (incluido si son punteros
//
// Si en la llamada RPC, la respuesta llega en un intervalo de tiempo,
// la funcion devuelve true, sino devuelve false
//
// la llamada RPC deberia tener un timout adecuado.
//
// Un resultado falso podria ser causado por una replica caida,
// un servidor vivo que no es alcanzable (por problemas de red ?),
// una petición perdida, o una respuesta perdida
//
// Para problemas con funcionamiento de RPC, comprobar que la primera letra
// del nombre  todo los campos de la estructura (y sus subestructuras)
// pasadas como parametros en las llamadas RPC es una mayuscula,
// Y que la estructura de recuperacion de resultado sea un puntero a estructura
// y no la estructura misma.
func (nr *NodoRaft) enviarPeticionVoto(nodo int, args *ArgsPeticionVoto,
	reply *RespuestaPeticionVoto) bool {
	timeout := 500 * time.Millisecond
	nr.Logger.Println("ESTOY EN ENVIARPETICIONVOTO")
	err := nr.Nodos[nodo].CallTimeout("NodoRaft.PedirVoto", args, &reply, timeout)
	if err != nil {
		nr.Logger.Println("Error en PedirVoto")
		return false
	} else if reply.Term > nr.CurrentTerm {
		//si hay nodo con mandato más alto => seguidor
		nr.Logger.Println("Recibido una respuesta con mayor mandato")
		nr.Estado = 0
		return false
	} else if (reply.Term <= nr.CurrentTerm) && reply.VoteGranted {
		nr.Logger.Println("Me han votado")
		//si responden y la votación es de mi mandato
		nr.VotesRec++
		if nr.VotesRec > len(nr.Nodos)/2 && nr.Estado == 1 {
			//recibe votos de la mayoría de nodos => líder
			nr.Estado = 2
			nr.Logger.Println("Soy lider")
			for i := 0; i <= len(nr.Nodos)-1; i++ {
				if i != nr.Yo {
					nr.NextIndex[i] = len(nr.Log)
					nr.MatchIndex[i] = -1
				}
				nr.enviar(i)
			}
		}
	}
	return true
}
