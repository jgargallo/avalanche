package lines

import (
	"github.com/gorilla/websocket"
	"sync"
	"time"
	"fmt"
)

const (
	TurnsPoolsCapacity = 100
	ConnsCapacityPerTurnsPool = 2 //TODO right capacity? (balance between goroutines and iterations per goroutine)
	DefaultAccessMaxAge = 600
	DefaultWaitingMaxAge = 15
	DefaultLineCapacity = 2 // clients accessing resource at the same time
)

type TurnsPool struct {
	conns []*websocket.Conn
}

func NewTurnsPool() *TurnsPool {
	return &TurnsPool{
		conns: make([]*websocket.Conn, 0, ConnsCapacityPerTurnsPool),
	}
}

func (pool *TurnsPool) AppendTurnConn(conn *websocket.Conn) {
	pool.conns = append(pool.conns, conn)
}

func (pool *TurnsPool) IsFull() bool {
	fmt.Printf("Curr pool capacity: %v of %v", len(pool.conns), cap(pool.conns))
	return len(pool.conns) == cap(pool.conns)
}

type Line struct {
	id string
	accessMaxAge uint32 // max time accessing resource (starts counting after getting the Token)
	waitingMaxAge uint32 // max time waiting to access resource (between access given and actually getting the Token)
	capacity uint32

	nextTurn uint32 //TODO to be replaced by Redis, we cannot cache this
	nextTurnMux sync.Mutex

					//TODO to be replaced by Redis, we cannot cache this
	nextIn uint32 // Next one to get access to the resource
	nextInMux sync.Mutex

	//TODO sorted set queue to control expiration

	pools []*TurnsPool
	currentPool *TurnsPool
	appendConnMux sync.Mutex
}

//TODO health check: size of sorted set <= line.capacity

func NewLine(id string) *Line {

	line := &Line{
		id: id,
		accessMaxAge: DefaultAccessMaxAge,
		waitingMaxAge: DefaultWaitingMaxAge,
		capacity: DefaultLineCapacity,
		nextTurn: 0, //TODO to be replaced by Redis impl
		nextIn: DefaultLineCapacity, // TODO to be replaced by Redis impl
		pools: make([]*TurnsPool, 1, TurnsPoolsCapacity),
	}

	//TODO initialize sorted set

	//TODO goroutine for handling expiration

	line.newTurnsPool()

	go line.broadcastNextIn(line.currentPool)

	return line
}

func (line *Line) GetId() string {
	return line.id
}

func (line *Line) GetAccessMaxAge() uint32 {
	return line.accessMaxAge
}

func (line *Line) GetWaitingMaxAge() uint32 {
	return line.waitingMaxAge
}

// Mutex is required for memory implementation, when using Redis will be removed
// since Redis is mono-thread
func (line *Line) IncNextTurn() uint32 {
	line.nextTurnMux.Lock()
	line.nextTurn++
	next := line.nextTurn
	line.nextTurnMux.Unlock()

	return next
}

func (line *Line) GetNextIn() uint32 {
	//TODO from redis
	return line.nextIn
}

func (line *Line) IsAccessGranted(turn uint32) bool {
	return turn <= line.GetNextIn()
}

func (line *Line) newTurnsPool()  {
	pool := NewTurnsPool()
	line.pools = append(line.pools, pool)
	line.currentPool = pool
}

func (line *Line) ReleaseTurn(turn uint32) uint32 {
	//TODO remove turn from redis sorted set with ZREM
	//TODO check ZREM result, line.nextIn++ only when ZREM result == 1
	//TODO this whole operation must be atomic so LUA script required


	line.nextInMux.Lock()
	line.nextIn++
	nextIn := line.nextIn
	line.nextInMux.Unlock()

	//TODO zadd nextIn with now + waitingMaxAge as score

	return nextIn
}

func (line *Line) AppendTurnConn(conn *websocket.Conn)  {
	line.appendConnMux.Lock()
	isNewPool := line.currentPool.IsFull()
	var newPool *TurnsPool
	if isNewPool {
		line.newTurnsPool()
		newPool = line.currentPool
	}
	line.currentPool.AppendTurnConn(conn)
	line.appendConnMux.Unlock()

	if isNewPool {
		go line.broadcastNextIn(newPool)
	}
}

func (line *Line) broadcastNextIn(turnsPool *TurnsPool) {
	for {
		fmt.Printf("Pool %v::%p -> NextTurn: %v, NextIn: %v\n",
			line.id, turnsPool, line.nextTurn, line.GetNextIn())
		for _, conn := range turnsPool.conns {
			conn.WriteMessage(1, []byte(fmt.Sprint(line.GetNextIn())))
		}
		fmt.Printf("***********************\n")
		time.Sleep(5000 * time.Millisecond)
	}
}
