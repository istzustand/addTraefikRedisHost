package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"

	"github.com/go-redis/redis"
)

type config struct {
	Endpoint        endpointConfig `json:"endpoint"`
	Redis           redisConfig    `json:"redis"`
	cfg             string
	printExampleCfg bool
	del             bool
}

type endpointConfig struct {
	Domain string `json:"domain"`
	Host   string `json:"host"`
	IP     string `json:"ip"`
}

type redisConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
	User string `json:"user"`
	Pwd  string `json:"pwd"`
}

func main() {
	flagcfg := parseFlags()
	if flagcfg.printExampleCfg {
		printExampleCfg()
		return
	}
	filecfg := readConfig(flagcfg.cfg)
	cfg := mergeConfigs(*flagcfg, *filecfg)
	err := verifyCfg(cfg)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("The following configuration is being used:")
	pp(cfg)

	bkey := "traefik/http/services/" + cfg.Endpoint.Host + "/loadBalancer/servers/0/url"
	skey := "traefik/http/routers/" + cfg.Endpoint.Host + "/service"
	rkey := "traefik/http/routers/" + cfg.Endpoint.Host + "/rule"

	bval := "http://" + cfg.Endpoint.IP
	sval := cfg.Endpoint.Host
	rval := "Host(`" + cfg.Endpoint.Host + "." + cfg.Endpoint.Domain + "`)"

	rc := connectToRedis(cfg)

	// probably not nessescary but I had a not reproducable bug where traefik didn't update till keys were deleted and set afterwards...
	rc.Del(bkey)
	rc.Del(skey)
	rc.Del(rkey)

	if !cfg.del {
		rc.Set(bkey, bval, 0)
		rc.Set(skey, sval, 0)
		rc.Set(rkey, rval, 0)
	}
}

func connectToRedis(cfg config) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Host + ":" + strconv.Itoa(cfg.Redis.Port),
		Password: cfg.Redis.Pwd,
		DB:       0,
	})
}

func verifyCfg(cfg config) error {
	if cfg.Endpoint.Domain == "" {
		return errors.New("Endpoint domain not specified!")
	}
	if cfg.Endpoint.Host == "" {
		return errors.New("Endpoint host not specified!")
	}
	if cfg.Endpoint.IP == "" {
		return errors.New("Endpoint IP not specified!")
	}
	if cfg.Redis.Host == "" {
		return errors.New("Redis host not specified!")
	}
	if cfg.Redis.Port == 0 {
		return errors.New("Redis port not specified!")
	}

	return nil
}

func printExampleCfg() {
	var c config
	c.Redis.Host = "1.2.3.4"
	c.Redis.Port = 6379
	c.Redis.User = "username"
	c.Redis.Pwd = "password"

	c.Endpoint.Host = "host"
	c.Endpoint.Domain = "example.com"
	c.Endpoint.IP = "1.2.3.4"
	s, _ := json.MarshalIndent(c, "", "  ")
	fmt.Println(string(s))
}

func mergeConfigs(fc config, jc config) config {
	var c config

	c.Endpoint.Domain = strMerge(fc.Endpoint.Domain, jc.Endpoint.Domain)
	c.Endpoint.Host = strMerge(fc.Endpoint.Host, jc.Endpoint.Host)
	c.Endpoint.IP = strMerge(fc.Endpoint.IP, jc.Endpoint.IP)

	c.Redis.Host = strMerge(fc.Redis.Host, jc.Redis.Host)
	c.Redis.Pwd = strMerge(fc.Redis.Pwd, jc.Redis.Pwd)
	c.Redis.User = strMerge(fc.Redis.User, jc.Redis.User)

	if jc.Redis.Port != 0 {
		c.Redis.Port = jc.Redis.Port
	} else {
		c.Redis.Port = fc.Redis.Port
	}

	c.del = fc.del

	return c
}

func strMerge(dom string, sub string) string {
	var res string
	if dom != "" {
		res = dom
	} else {
		res = sub
	}
	return res
}

func parseFlags() *config {
	var c config
	flag.StringVar(&c.Redis.Host, "rh", "", "the hostname or IP of the redis server storing traefik-info")
	flag.IntVar(&c.Redis.Port, "rP", 6732, "Port of the redis-db. Standard-port is assumed if none is set")
	flag.StringVar(&c.Redis.User, "ru", "", "login for redis")
	flag.StringVar(&c.Redis.Pwd, "rp", "", "redis password")

	flag.StringVar(&c.Endpoint.Domain, "ed", "", "domain of the endpoint")
	flag.StringVar(&c.Endpoint.Host, "eh", "", "hostname of the service")
	flag.StringVar(&c.Endpoint.IP, "ei", "", "IP-address for the service")

	flag.StringVar(&c.cfg, "cfg", "cfg.json", "config-file")
	flag.BoolVar(&c.printExampleCfg, "cfgxmpl", false, "print an example config-file")
	flag.BoolVar(&c.del, "rm", false, "if set the given service is removed instead of created")
	flag.Parse()

	return &c
}

func readConfig(file string) *config {
	var j config

	f, err := os.Open(file)
	if err != nil {
		log.Println("No config file provided, relying on cli-arguments", err)
		return &j
	}
	defer f.Close()

	b, err := ioutil.ReadAll(f)
	if err != nil {
		log.Fatal("Error reading the config file ", err)
	}
	err = json.Unmarshal(b, &j)
	if err != nil {
		log.Fatal("Error parsing the config file ", err)
	}
	return &j
}

func pp(v any) {
	d, _ := json.MarshalIndent(v, "", "  ")

	fmt.Println(string(d))
}
