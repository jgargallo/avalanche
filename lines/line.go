package lines

import (
	"github.com/gorilla/websocket"
	"sync"
)

const (
	TurnsPoolsCapacity = 100
	ConnsCapacity = 1000
)


type TurnConn struct {
	turn uint32
	conn *websocket.Conn
}

type TurnsPool struct {
	conns []*TurnConn
}

func NewTurnsPool() *TurnsPool {
	return &TurnsPool{
		conns: make([]*TurnConn, 0, ConnsCapacity),
	}
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
	nextPoolMux sync.Mutex
}

func NewLine(id string) *Line {
	line := &Line{
		id: id,
		nextTurn: 0, //TODO to be replaced by Redis impl
		nextIn: 1, // TODO to be replaced by Redis impl
		pools: make([]*TurnsPool, 1, TurnsPoolsCapacity),
	}

	line.newTurnsPool()

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
	line.nextPoolMux.Lock()
	pool := NewTurnsPool()
	line.pools = append(line.pools, pool)
	line.currentPool = pool
	line.nextPoolMux.Unlock()
}

func (line *Line) ReleaseTurn() uint32 {
	line.nextInMux.Lock()
	line.nextIn++
	nextIn := line.nextIn
	line.nextInMux.Unlock()

	return nextIn
}
