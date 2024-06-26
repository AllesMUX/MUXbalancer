package servers

import (
    "fmt"
    "log"
    "strings"
    "strconv"
    "sync"
    "time"
    "encoding/json"
    "github.com/valyala/fasthttp"
    "github.com/go-redis/redis"
    MUXerrors "github.com/AllesMUX/MUXbalancer/errors"
    MUXworkerStructs "github.com/AllesMUX/MUXworker/structs" 
)

type Server struct {
    Key        string `json:"key"`
    Protocol   string `json:"protocol"`
    Addr       string `json:"addr"`
    Port       string `json:"port"`
    WorkerPort string `json:"worker_port"`
}

type ServerManager struct {
    RedisOptions *redis.Options
    redisClient *redis.Client
    servers []*Server
}

type ServerHealthStatus struct {
    Server Server `json:"server"`
    Health MUXworkerStructs.ServerStatusJSON `json:"health"`
    Online bool `json:"online"`
}

func isOnline(endpoint string) bool {
    req := fasthttp.AcquireRequest()
    resp := fasthttp.AcquireResponse()
    defer fasthttp.ReleaseRequest(req)
    defer fasthttp.ReleaseResponse(resp)
    req.SetRequestURI(endpoint)
    err := fasthttp.Do(req, resp)
    if err != nil {
        return false
    }
    if resp.StatusCode() != fasthttp.StatusOK {
        return false
    }
    return true
}


func (sm *ServerManager) askAllServersHealth() chan *ServerHealthStatus {
    ch := make(chan *ServerHealthStatus)
    wg := sync.WaitGroup{}
    for _, srv := range sm.servers {
        wg.Add(1)
        go func(srv Server) {
            defer wg.Done()
            r, err := sm.GetServerHealth(srv, "server-health")
            s := ServerHealthStatus{
                Server: srv,
                Health: r,
                Online: err == nil,
            }
            ch <- &s
        }(*srv)
    }
    go func() {
        wg.Wait()
        close(ch)
    }()
    return ch
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
func (sm *ServerManager) GetServersWithHealth() []ServerHealthStatus  {
    var servers []ServerHealthStatus
    for server := range sm.askAllServersHealth() {
        servers = append(servers, *server)
    }
    return servers
}

func (sm *ServerManager) AddServer(s *Server) error {
    for _, server := range sm.servers {
        if server.Addr == s.Addr && server.Port == s.Port {
            return MUXerrors.ServerExists
        }
    }
    allServerKeys, _, _ := sm.redisClient.Scan(0, "server:*", 0).Result()
    var lastKey int
    if len(allServerKeys) > 0 {
        // Find the maximum key value
        for _, key := range allServerKeys {
            keyValue, err := strconv.Atoi(strings.Split(key, ":")[1])
            if err != nil {
                log.Println("Error parsing server key:", err)
                continue
            }
            if keyValue > lastKey {
                lastKey = keyValue
            }
        }
    } else {
        lastKey = -1
    }
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
    s.Key = key
    sm.servers = append(sm.servers, s)
    return nil
}

func (sm *ServerManager) RemoveServer(key string) {
    var index int
    var found bool
    for i, server := range sm.servers {
        if server.Key == key {
            index = i
            found = true
            break
        }
    }
    if !found {
        return
    }
    sm.servers = append(sm.servers[:index], sm.servers[index+1:]...)
    sm.redisClient.Del(key)
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

func (sm *ServerManager) GetServerHealth(s Server, endpoint string) (MUXworkerStructs.ServerStatusJSON, error) {
    url := fmt.Sprintf("%s://%s:%s/%s", s.Protocol, s.Addr, s.WorkerPort, endpoint)
    
    client := &fasthttp.Client{
        MaxConnDuration: 1 * time.Second,
    }

    req := fasthttp.AcquireRequest()
    defer fasthttp.ReleaseRequest(req)
    req.SetRequestURI(url)
    req.Header.SetMethod("GET")

    resp := fasthttp.AcquireResponse()
    defer fasthttp.ReleaseResponse(resp)

    var r MUXworkerStructs.ServerStatusJSON
    
    err := client.Do(req, resp)
    if err != nil {
        return r, err
    }
    
    err = json.Unmarshal(resp.Body(), &r)
    if err != nil {
        return r, err
    }
    return r, nil
}

func (sm *ServerManager) GetLowestLoadedServer() *Server {
    ch := sm.askAllServersHealth()
    lowestLoaded := ServerHealthStatus{}
    var tmp = false
    for res := range ch {
        if tmp == false {
            lowestLoaded = *res
            tmp = true
        } else {
            if lowestLoaded.Health.ActiveTasks > res.Health.ActiveTasks {
                lowestLoaded = *res
            } else if lowestLoaded.Health.CPULoadAvg > res.Health.CPULoadAvg {
                lowestLoaded = *res
            }
        }
    }
    return &lowestLoaded.Server
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

