package main

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"bitbucket.org/jgargallo/avalanche/lines"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

func main() {

	r := gin.Default()
	r.LoadHTMLFiles("index.html")

	r.GET("/lines/:resource", func(c *gin.Context) {
		c.HTML(200, "index.html", nil)
	})

	r.POST("/lines/:resource/nextTurn", NextTurn) // request for a new turn
	r.GET("/lines/:resource/nextIn", NextIn) // webSocket to be updated with next turn allowed to get in
	r.GET("/lines/:resource/token", GetToken) // access granted, time to request for the token to access resource
	r.GET("/lines/:resource/release/:turn", ReleaseResource) // releases resource, nextIn incremented

	r.Run("localhost:12312")
}

const (
	TurnCookieName = "av_t"
	SignedTurnCookieName = "av_tsig"
	CookieMaxAge = 864000 // 10 days
)

var cachedLines = make(map[string]*lines.Line)

var lineMux sync.Mutex

func getCachedLine(resource string) *lines.Line {
	line, cachedLine := cachedLines[resource]
	if !cachedLine { // double check to use mutex only when line not cached
		lineMux.Lock()
		_, c := cachedLines[resource]
		if !c {
			line = lines.NewLine(resource)
			cachedLines[resource] = line
		}
		lineMux.Unlock()
	}
	return line
}

func NextTurn(c *gin.Context) {
	resource := c.Param("resource")
	line := getCachedLine(resource)

	cookie, err := c.Request.Cookie(SignedTurnCookieName)
	var nextTurnCookie string
	if err != nil {
		nextTurn := line.IncNextTurn()
		cookiePath := fmt.Sprintf("/lines/%v", resource)
		nextTurnCookie = fmt.Sprint(nextTurn)
		if nextTurn <= line.GetNextIn() {
			c.SetCookie(TurnCookieName, nextTurnCookie, CookieMaxAge, cookiePath, "", false, false)
			processGetToken(c, line, nextTurn)
			return
		} else {

			// TODO created cookie with signed turn
			c.SetCookie(TurnCookieName, nextTurnCookie, CookieMaxAge, cookiePath, "", false, false)
			c.SetCookie(SignedTurnCookieName, nextTurnCookie, CookieMaxAge, cookiePath, "", false, true)
		}
	} else {
		nextTurnCookie = cookie.Value
		// TODO If existing cookie with past turn: overwrite cookie with new turn
	}

	c.JSON(200, gin.H{
		"resource": resource,
		"turn": nextTurnCookie,
		"status": "waiting",
	})
}

var wsupgrader = websocket.Upgrader{
	ReadBufferSize:  32,
	WriteBufferSize: 32,
}


func wsHandler(w http.ResponseWriter, r *http.Request, resource string) {
	conn, err := wsupgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Failed to set websocket upgrade: %+v", err)
		return
	}

	// TODO use control messages instead
	conn.ReadMessage()

	line := getCachedLine(resource)
	line.AppendTurnConn(conn)
}

func NextIn(c *gin.Context) {
	wsHandler(c.Writer, c.Request, c.Param("resource"))
}

func GetToken(c *gin.Context) {
	resource := c.Param("resource")

	cookie, err := c.Request.Cookie(SignedTurnCookieName)
	if err != nil {
		c.JSON(400, gin.H{
			"resource": resource,
			"message": "No turn requested.",
		})
		return
	}

	turn64, err := strconv.ParseUint(cookie.Value, 10, 32) // TODO parse and check signature
	if err != nil {
		// TODO assumed token has already been delivered
		c.JSON(200, gin.H{
			"status": "access_granted",
			"turn": strings.Split(cookie.Value, "%2C")[0],
		})
		return
	}
	turn := uint32(turn64)

	// TODO Check if token already delivered, if so return 200 and do nothing

	line := getCachedLine(resource)
	nextIn := line.GetNextIn()
	if turn > nextIn {
		c.JSON(400, gin.H{
			"resource": resource,
			"message": "Not your turn yet. You should wait until access granted.",
		})
		return
	}

	processGetToken(c, line, turn)
}

func processGetToken(c *gin.Context, line *lines.Line, turn uint32) {
	cookiePath := fmt.Sprintf("/lines/%v", line.GetId())
	c.SetCookie(SignedTurnCookieName, fmt.Sprintf("%v,IN", turn), int(line.GetAccessMaxAge()),
		cookiePath, "", false, true)
	//TODO update sorted set

	c.JSON(200, gin.H{
		"resource": line.GetId(),
		"turn": turn,
		"status": "access_granted",
	})
}

func ReleaseResource(c *gin.Context) {
	resource := c.Param("resource")
	turn := c.Param("turn")
	turn64, err := strconv.ParseUint(turn, 10, 32)
	if err != nil {
		c.JSON(400, gin.H{
			"resource": resource,
			"message": "Invalid turn",
		})
		return
	}

	line := getCachedLine(resource)
	nextIn := line.ReleaseTurn(uint32(turn64))
	c.JSON(200, gin.H{
		"resource": resource,
		"next_in": nextIn,
	})
}



