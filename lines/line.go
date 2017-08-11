package lines

import (
	"github.com/gorilla/websocket"
	"sync"
	"time"
	"fmt"
)

const (
	TurnsPoolsCapacity = 100
	ConnsCapacity = 1
)

type TurnsPool struct {
	conns []*websocket.Conn
}

func NewTurnsPool() *TurnsPool {
	return &TurnsPool{
		conns: make([]*websocket.Conn, 0, ConnsCapacity),
	}
}

func (pool *TurnsPool) AppendTurnConn(conn *websocket.Conn) {
	pool.conns = append(pool.conns, conn)
}

func (pool *TurnsPool) IsFull() bool {
	return len(pool.conns) == cap(pool.conns)
}

type Line struct {
	id string

	nextTurn uint32 //TODO to be replaced by Redis, we cannot cache this
	nextTurnMux sync.Mutex

					//TODO to be replaced by Redis, we cannot cache this
	nextIn uint32 // Next one to get access to the resource
	nextInMux sync.Mutex

	pools []*TurnsPool
	currentPool *TurnsPool
	appendConnMux sync.Mutex
}

func NewLine(id string) *Line {

	line := &Line{
		id: id,
		nextTurn: 0, //TODO to be replaced by Redis impl
		nextIn: 1, // TODO to be replaced by Redis impl
		pools: make([]*TurnsPool, 1, TurnsPoolsCapacity),
	}

	line.newTurnsPool()

	go line.broadcastNextIn(line.currentPool)

	return line
}

// Mutex is required for memory implementation, when using Redis will be removed
// since Redis is mono-thread
func (line *Line) GetNextTurn() uint32 {
	line.nextTurnMux.Lock()
	line.nextTurn++
	next := line.nextTurn
	line.nextTurnMux.Unlock()

	return next
}

func (line *Line) NextIn() uint32 {
	return line.nextIn
}

func (line *Line) newTurnsPool()  {
	pool := NewTurnsPool()
	line.pools = append(line.pools, pool)
	line.currentPool = pool
}

func (line *Line) ReleaseTurn() uint32 {
	line.nextInMux.Lock()
	line.nextIn++
	nextIn := line.nextIn
	line.nextInMux.Unlock()

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
		fmt.Printf("%p -> %v\n", turnsPool, line.NextIn())
		for _, conn := range turnsPool.conns {
			conn.WriteMessage(1, []byte(fmt.Sprint(line.NextIn())))
		}
		time.Sleep(3000 * time.Millisecond)
	}
}
