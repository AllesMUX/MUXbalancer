package main

import (
	"fmt"
	"sync"
	"time"
	"log"
	"os"

	"encoding/json"
	
	"github.com/google/uuid"

	"github.com/valyala/fasthttp"
	"github.com/go-redis/redis"

	"github.com/AllesMUX/MUXbalancer/servers"
	"github.com/AllesMUX/MUXbalancer/config"
)

var appConfig = config.GetConfig("config.yaml") 


type session struct {
    server *servers.Server
    expiry time.Time
}

var sessions = struct {
    sync.RWMutex
    m map[string]*session
}{m: make(map[string]*session)}

func getSession(ctx *fasthttp.RequestCtx) *session {
    sessions.RLock()
    sess, ok := sessions.m[string(ctx.Request.Header.Cookie(appConfig.App.Cookie))]
    sessions.RUnlock()
    if !ok || sess.expiry.Before(time.Now()) {
        return nil
    }
    return sess
}

func setSession(ctx *fasthttp.RequestCtx, sess *session) {
    sessions.Lock()
    sessions.m[string(ctx.Request.Header.Cookie(appConfig.App.Cookie))] = sess
    sessions.Unlock()
}

func main() {
    workingServers := servers.ServerManager{
        RedisOptions:&redis.Options{
            Addr: appConfig.Redis.Addr,
            Password: appConfig.Redis.Password,
            DB: appConfig.Redis.DB,
        },
    }

    err := workingServers.LoadServers()
    if err != nil {
        panic(err)
    }
    /*
    s := workingServers.GetLowestLoadedServer()
    println(s.Port)
    */
    /*
    serverHealth := workingServers.GetServerHealth(*workingServers.GetServerByIndex(0), appConfig.Worker.HealthEndpoint)
    fmt.Println(serverHealth.CPULoadAvg)
    */
    /*
    err = workingServers.AddServer(&servers.Server{
        Key: "testt",
        Protocol: "http",
        Addr: "127.0.0.1",
        Port: "8083",
    })
    if err != nil {
        panic(err)
    }
    workingServers.InitTestServers()
    */
    api := func(ctx *fasthttp.RequestCtx) {
        reqToken := ctx.Request.Header.Peek("Authorization")
        if string(reqToken) != "Bearer "+appConfig.API.Token  {
            ctx.SetStatusCode(fasthttp.StatusUnauthorized)
            ctx.SetContentType("application/json")
            ctx.WriteString(`{"error":"invalid API key"}`)
            return
        }
        
        
        var response map[string]interface{}
    
        message := string(ctx.QueryArgs().Peek("method"))
        //data := string(ctx.QueryArgs().Peek("data"))
        switch message {
            case "servers_list":
                response = map[string]interface{}{
                    "data": workingServers.GetServers(),
                }
            case "servers_list_health":
                response = map[string]interface{}{
                    "data": workingServers.GetServersWithHealth(),
                }
            default:
                response = map[string]interface{}{
                    "error": "invalid type",
                }
        }
    
        jsonResponse, err := json.Marshal(response)
        if err != nil {
            ctx.SetStatusCode(fasthttp.StatusInternalServerError)
            ctx.SetContentType("application/json")
            ctx.WriteString(fmt.Sprintf(`{"error":"server error - %s"}`, err))
            return
        }
    
        ctx.SetStatusCode(fasthttp.StatusOK)
        ctx.SetContentType("application/json")
        ctx.Write(jsonResponse)
    }
    log.Printf("MUXbalancer API starting on http://0.0.0.0:%d\n", appConfig.API.Port)
    go fasthttp.ListenAndServe(fmt.Sprintf(":%d",appConfig.API.Port), api)

    var current int
    lb := func(ctx *fasthttp.RequestCtx) {
        cookie := ctx.Request.Header.Cookie(appConfig.App.Cookie)
        if len(cookie) == 0 {
            var c fasthttp.Cookie
	        c.SetKey(appConfig.App.Cookie)
	        c.SetValue(uuid.New().String())
            ctx.Response.Header.SetCookie(&c)
        }
        
        sess := getSession(ctx)
        if sess == nil {
            srv := workingServers.GetServerByIndex(current)
            current = (current + 1) % workingServers.GetServersCount()
            sess = &session{server: srv, expiry: time.Now().Add(time.Duration(appConfig.App.SessionLifetime * int(time.Second)))}
            setSession(ctx, sess)
            fmt.Printf("New user using RR balance. Selected server is %s:%s\n", sess.server.Addr, sess.server.Port)
        }
        needLL := false
        for _, b := range appConfig.Worker.Balance {
            if b.Path == string(ctx.Path()) && b.Method == string(ctx.Method()) {
                needLL = true
                break
            }
        }
        if needLL {
            sess.server = workingServers.GetLowestLoadedServer()
            fmt.Printf("New balancing request using LL balance. Selected server is %s:%s\n", sess.server.Addr, sess.server.Port)
        }
        serverURI := sess.server.Addr + ":" + sess.server.Port
        var proxyClient = &fasthttp.HostClient{
		  Addr:                   serverURI,
		  IsTLS:                  sess.server.Protocol == "https",
		  ReadBufferSize:         8192,
	    }
        if err := proxyClient.Do(&ctx.Request, &ctx.Response); err != nil {
            ctx.Error(fmt.Sprintf("Internal Server Error: %s", err), fasthttp.StatusInternalServerError)
            return
        }
    }
    
    if appConfig.App.Serve == "http" {
        log.Printf("MUXbalancer starting on http://0.0.0.0:%d\n", appConfig.App.Port)
        if err := fasthttp.ListenAndServe(fmt.Sprintf(":%d",appConfig.App.Port), lb); err != nil {
            log.Fatalf("error in fasthttp server: %s", err)
        }
    } else if appConfig.App.Serve == "socket" {
	    log.Printf("MUXbalancer starting on http://unix:%s\n", appConfig.App.Socket)
        if _, err := os.Stat(appConfig.App.Socket); err == nil {
            os.Remove(appConfig.App.Socket)
        }
        if err := fasthttp.ListenAndServeUNIX(appConfig.App.Socket, 0777, lb); err != nil {
            log.Fatalf("error in fasthttp server: %s", err)
        }
        defer os.Remove(appConfig.App.Socket)
    } else {
        log.Fatalf("Unknown serve method %s. Use http or socket.", appConfig.App.Serve)
    }
}
