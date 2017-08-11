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

	r.POST("/lines/:resource/nextTurn", NextTurn)
	r.GET("/lines/:resource/nextIn", NextIn)
	r.GET("/lines/:resource/token", GetToken)
	r.GET("/lines/:resource/release", ReleaseResource)

	r.Run("localhost:12312")
}

const (
	TurnCookieName = "av_t"
	SignedTurnCookieName = "av_tsig"
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
	var nextTurn string
	if err != nil {
		nextTurn = fmt.Sprint(line.GetNextTurn())
		cookiePath := fmt.Sprintf("/lines/%v", resource)
		// TODO created cookie with signed turn
		c.SetCookie(TurnCookieName, nextTurn, 600, cookiePath, "", false, false)
		c.SetCookie(SignedTurnCookieName, nextTurn, 600, cookiePath, "", false, true)
	} else {
		// TODO If existing cookie with valid turn: do nothing
		// TODO If existing cookie with past turn: overwrite cookie with new turn
		nextTurn = cookie.Value
	}

	c.JSON(200, gin.H{
		"resource": resource,
		"turn": nextTurn,
	})
}

var wsupgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}


func wsHandler(w http.ResponseWriter, r *http.Request, resource string) {
	conn, err := wsupgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Failed to set websocket upgrade: %+v", err)
		return
	}

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
	nextIn := line.NextIn()
	if turn > nextIn {
		c.JSON(400, gin.H{
			"resource": resource,
			"message": "Not your turn yet. You should wait until access granted.",
		})
		return
	}

	cookiePath := fmt.Sprintf("/lines/%v", resource)
	c.SetCookie(SignedTurnCookieName, fmt.Sprintf("%v,IN", cookie.Value), 600, cookiePath, "", false, true)

	c.JSON(200, gin.H{
		"status": "access_granted",
		"turn": cookie.Value,
	})
}

func ReleaseResource(c *gin.Context) {
	resource := c.Param("resource")
	line := getCachedLine(resource)
	nextIn := line.ReleaseTurn()
	c.JSON(200, gin.H{
		"resource": resource,
		"next_in": nextIn,
	})
}



