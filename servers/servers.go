package servers

import (
    "fmt"
    "log"
    "strings"
    "strconv"
    "sync"
    "encoding/json"
    "github.com/valyala/fasthttp"
    "github.com/go-redis/redis"
    MUXerrors "github.com/AllesMUX/MUXbalancer/errors"
    MUXworkerStructs "github.com/AllesMUX/MUXworker/structs" 
)

type Server struct {
    Key        string
    Protocol   string
    Addr       string
    Port       string
    WorkerPort string
}

type ServerManager struct {
    RedisOptions *redis.Options
    redisClient *redis.Client
    servers []*Server
}

type ServerHealthStatus struct {
    server Server
    health MUXworkerStructs.ServerStatusJSON
}

func (sm *ServerManager) GetServersCount() int {
    return len(sm.servers)
}

func (sm *ServerManager) GetServerByIndex(index int) *Server {
    return sm.servers[index]
}

func (sm *ServerManager) GetServers() []*Server {
    return sm.servers
}

func (sm *ServerManager) AddServer(s *Server) error {
    for _, server := range sm.servers {
        if server.Addr == s.Addr && server.Port == s.Port {
            return MUXerrors.ServerExists
        }
    }
    allServerKeys, _, _ := sm.redisClient.Scan(0, "server:*", 0).Result()
    lastKey, _ := strconv.Atoi(strings.Split(allServerKeys[len(allServerKeys)-1], ":")[1])
    key := fmt.Sprintf("server:%d", lastKey+1)
    serverMap := map[string]interface{}{
        "protocol":    s.Protocol,
        "addr":        s.Addr,
        "port":        s.Port,
        "worker_port": s.WorkerPort,
    }
    err := sm.redisClient.HMSet(key, serverMap).Err()
    if err != nil {
        log.Println("Can't add server in Redis:", err)
        return err
    }
    sm.servers = append(sm.servers, s)
    return nil
}

func (sm *ServerManager) RemoveServer(key string) {
    var index int
    for i, server := range sm.servers {
        if server.Key == key {
            index = i
            break
        }
    }
    if index == len(sm.servers) {
        return
    }
    sm.servers = append(sm.servers[:index], sm.servers[index+1:]...)
}

func (sm *ServerManager) LoadServers() error {
    sm.redisClient = redis.NewClient(sm.RedisOptions)
    iter := sm.redisClient.Scan(0, "server:*", 0).Iterator()
    for iter.Next() {
        result, err := sm.redisClient.HGetAll(iter.Val()).Result()
        if err != nil {
            log.Println("Can't load servers from Redis:", err)
            return err
        }
        if(result != nil) {
            server := Server{
                Key: iter.Val(),
                Protocol: result["protocol"],
                Addr: result["addr"],
                Port: result["port"],
                WorkerPort: result["worker_port"],
            }
            sm.servers = append(sm.servers, &server)
        }
    }
    return nil
}

type httpResponse struct {
    url      string
    response *fasthttp.Response
    err      error
}

func (sm *ServerManager) GetServerHealth(s Server, endpoint string) MUXworkerStructs.ServerStatusJSON {
    url := fmt.Sprintf("%s://%s:%s/%s", s.Protocol, s.Addr, s.WorkerPort, endpoint)

    req := fasthttp.AcquireRequest()
    defer fasthttp.ReleaseRequest(req)

    req.SetRequestURI(url)
    req.Header.SetMethod("GET")

    resp := fasthttp.AcquireResponse()
    defer fasthttp.ReleaseResponse(resp)

    if err := fasthttp.Do(req, resp); err != nil {
        log.Fatalf("error in fasthttp request: %s", err)
    }

    var r MUXworkerStructs.ServerStatusJSON
    if err := json.Unmarshal(resp.Body(), &r); err != nil {
        log.Fatalf("error in json unmarshal: %s", err)
    }
    return r
}


func (sm *ServerManager) GetLowestLoadedServer() *Server {
    ch := make(chan *ServerHealthStatus)
    wg := sync.WaitGroup{}

    for _, srv := range sm.servers {
        wg.Add(1)
        go func(srv Server) {
            defer wg.Done()
            s := ServerHealthStatus{
                server: srv,
                health: sm.GetServerHealth(srv, "server-health"),
            }
            ch <- &s
        }(*srv)
    }

    go func() {
        wg.Wait()
        close(ch)
    }()

    lowestLoaded := ServerHealthStatus{}
    var tmp = false
    for res := range ch {
        if tmp == false {
            lowestLoaded = *res
            tmp = true
        } else {
            if lowestLoaded.health.ActiveTasks > res.health.ActiveTasks {
                lowestLoaded = *res
            } else if lowestLoaded.health.CPULoadAvg > res.health.CPULoadAvg {
                lowestLoaded = *res
            }
        }
    }
    return &lowestLoaded.server
}


func (s *Server) handleRequest(ctx *fasthttp.RequestCtx) {
    fmt.Printf("Server %s: %s %s\n", s.Addr, ctx.Method(), ctx.Path())
    ctx.SetStatusCode(fasthttp.StatusOK)
    ctx.SetBodyString("Hello from " + s.Port)
}


func (sm *ServerManager) InitTestServers() {
    for _, srv := range sm.servers {
        fmt.Printf("started server %s\n",srv.Port)
        go func(s *Server) {
            if err := fasthttp.ListenAndServe(s.Addr + ":" + s.Port, s.handleRequest); err != nil {
                panic(err)
            }
        }(srv)
    }
}

