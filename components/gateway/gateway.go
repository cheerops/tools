package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"

	"gitlab.aswangc.cn/dataant/tools/components/config"
	"gitlab.aswangc.cn/dataant/tools/utils"

	httptool "gitlab.aswangc.cn/dataant/tools/tools/http_tool"
)

/*
	添加服务到目标网关
*/

type ServiceConfig struct {
	ServiceKey string `yaml:"servicekey" json:"service_key"`
	Version    string `yaml:"version" json:"version"`
	Desc       string `yaml:"desc" json:"desc"`
	Proxy      string `yaml:"proxy" json:"host"`
	NeedAuth   bool   `yaml:"needauth" json:"need_auth"`
	AuthMode   string `yaml:"authmode" json:"auth_mode"`
	NeedLic    bool   `yaml:"needlic" json:"need_lic"`
}

// hosts:
//   - http://localhost:9001/proxy/add

// serviceslice:
//   - servicekey: serverkey
//     version: ver
//     desc: xxxxx
//     proxy: http://localhost:8888
//     needauth: false
//     authmode: service/web
//     needlic: false
type GatewayComponent struct {
	GatewayHosts []string
	ServiceSlice []ServiceConfig
}

// step: 加载配置中服务列表
func (gateway *GatewayComponent) Init() error {
	gateway.GatewayHosts = config.Cfg.GetStringSlice("gateway.hosts")
	var serviceSlice []ServiceConfig
	if err := config.Cfg.UnmarshalKey("gateway.serviceslice", &serviceSlice); err != nil {
		return err
	}
	gateway.ServiceSlice = serviceSlice

	return gateway.register()
}

func (gateway *GatewayComponent) register() error {
	for _, host := range gateway.GatewayHosts {
		jsonStr, _ := json.Marshal(gateway.ServiceSlice)
		header := http.Header{
			"Gateway-Apikey": {config.Cfg.GetString("gateway.apikey")},
		}
		resp, _, err := httptool.HttpJSON(utils.GetTraceId(), host, string(jsonStr), 5000, header)
		if err != nil {
			return err
		}
		if resp.StatusCode == http.StatusOK {
			fmt.Println("向网关注册服务：", host)
		} else {
			fmt.Println("向网关注册服务失败：", host)
		}
	}
	return nil
}
