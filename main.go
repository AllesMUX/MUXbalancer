package main

import (
	"fmt"
	"sync"
	"time"
	"log"
	"os"

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
        }

        ctx.Request.SetRequestURI(sess.server.Addr + string(ctx.Path()))
        fmt.Println(sess.server.Addr)
        req := fasthttp.AcquireRequest()
        
        defer fasthttp.ReleaseRequest(req)
    
        ctx.Request.CopyTo(req)
    
        req.SetRequestURI(sess.server.Protocol + "://" + sess.server.Addr + ":" + sess.server.Port)
    
        resp := fasthttp.AcquireResponse()
        defer fasthttp.ReleaseResponse(resp)
    
        if err := fasthttp.Do(req, resp); err != nil {
            ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
            return
        }
        resp.CopyTo(&ctx.Response)
    }
    
    if appConfig.App.Serve == "http" {
        log.Printf("Starting on http://0.0.0.0:%d\n", appConfig.App.Port)
        if err := fasthttp.ListenAndServe(fmt.Sprintf(":%d",appConfig.App.Port), lb); err != nil {
            log.Fatalf("error in fasthttp server: %s", err)
        }
    } else if appConfig.App.Serve == "socket" {
	    log.Printf("Starting on http://unix:%s\n", appConfig.App.Socket)
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
