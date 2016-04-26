package cron

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/gosexy/redis"
	"github.com/mesos-utility/redis-metrics/g"
	"github.com/open-falcon/common/model"
)

var gaugess = map[string]int{
	"connected_clients":        1,
	"blocked_clients":          1,
	"used_memory":              1,
	"used_memory_rss":          1,
	"used_memory_peak":         1,
	"mem_fragmentation_ratio":  1,
	"total_commands_processed": 0,
	"rejected_connections":     0,
	"expired_keys":             0,
	"evicted_keys":             0,
	"keyspace_hits":            0,
	"keyspace_misses":          0,
	"keyspace_hit_ratio":       1,
}

func Collect() {
	if !g.Config().Transfer.Enable {
		glog.Warningf("Open falcon transfer is not enabled!!!")
		return
	}

	if g.Config().Transfer.Addr == "" {
		glog.Warningf("Open falcon transfer addr is null!!!")
		return
	}

	addrs := g.Config().Daemon.Addrs
	if !g.Config().Daemon.Enable {
		glog.Warningf("Daemon collect not enabled in cfg.json!!!")

		if len(addrs) < 1 {
			glog.Warningf("Not set addrs of daemon in cfg.json!!!")
		}
		return
	}

	go collect(addrs)
}

func collect(addrs []string) {
	// start collect data for redis cluster.
	var attachtags = g.Config().AttachTags
	var interval int64 = g.Config().Transfer.Interval
	//var username = g.Config().Daemon.UserName
	var passwd = g.Config().Daemon.PassWord
	var tout = g.Config().Daemon.Timeout
	timeout := time.Duration(tout) * time.Second
	timer := time.NewTicker(time.Duration(interval) * time.Second)

	if len(g.Config().Metrics) > 0 {
		gaugess = g.Config().Metrics
	}
	glog.Infof("Collected metrics: %v", gaugess)

	for {
	REST:
		<-timer.C
		hostname, err := g.Hostname()
		if err != nil {
			goto REST
		}

		mvs := []*model.MetricValue{}
		for _, addr := range addrs {
			ip, port, err := net.SplitHostPort(addr)
			if err != nil {
				glog.Warningf("Error format addr of ip:port %s", err)
				continue
			}
			client := redis.New()
			portnum, _ := strconv.ParseUint(port, 10, 32)
			err = client.ConnectWithTimeout(ip, uint(portnum), timeout)
			if err != nil {
				glog.Warningf("Error connect for %s", addr)
				continue
			}
			//redis auth
			if passwd != "" {
				ss, _ := client.Auth(passwd)
				if ss != "OK" {
					glog.Warningf("Redis client auth failed for %s", addr)
					continue
				}
			}

			var secs = []string{"Server", "Clients", "Memory", "Stats", "Replication", "CPU"}
			stats := GetRedisInfo(client, secs...)
			client.Quit()

			stats["keyspace_hit_ratio"] = g.CalculateMetricRatio(stats["keyspace_hits"], stats["keyspace_misses"])

			var tags string
			if port != "" {
				tags = fmt.Sprintf("port=%s", port)
			}
			if attachtags != "" {
				tags = fmt.Sprintf("%s,%s", attachtags)
			}

			now := time.Now().Unix()
			var suffix, vtype string
			for k, v := range gaugess {
				if v == 1 {
					suffix = ""
					vtype = "GAUGE"
				} else {
					suffix = "_cps"
					vtype = "COUNTER"
				}

				value, ok := stats[k]
				if !ok {
					continue
				}

				key := fmt.Sprintf("redis.%s%s", k, suffix)

				metric := &model.MetricValue{
					Endpoint:  hostname,
					Metric:    key,
					Value:     value,
					Timestamp: now,
					Step:      interval,
					Type:      vtype,
					Tags:      tags,
				}

				mvs = append(mvs, metric)
				//glog.Infof("%v\n", metric)
			}
		}
		g.SendMetrics(mvs)
	}
}

func GetRedisInfo(c *redis.Client, secs ...string) map[string]string {
	stats := make(map[string]string)
	for _, sec := range secs {
		s, err := c.Info(sec)
		if err != nil {
			glog.Warningf("Get stats failure for section: %s", sec)
			continue
		}

		a1 := strings.Split(s, "\r\n")
		for i := 1; i < len(a1); i++ {
			if !strings.Contains(a1[i], ":") {
				continue
			}
			a2 := strings.Split(a1[i], ":")
			stats[a2[0]] = a2[1]
		}
	}
	return stats
}
