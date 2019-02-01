# HA system to handle virtual queues #

User experience when entering virtual queues for high demand events, let's say on-sales, it's always frustating. Users always feel they are at a random queue.

Big challenge here is to broadcast to all users (millions) the current status in the queue

Since I wanted to learn Go, this is a great use case to try out.

### Services exposed [WIP] ###

* POST "/lines/:resource/nextTurn" // request for a new turn
*	GET "/lines/:resource/nextIn" // webSocket to be updated with next turn allowed to get in
*	GET "/lines/:resource/token" // access granted, time to request for the token to access resource
*	GET "/lines/:resource/release/:turn" // releases resource, nextIn incremented
