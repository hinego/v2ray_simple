package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/asaskevich/govalidator"
	"github.com/hahahrfool/v2ray_simple/proxy"
	"github.com/hahahrfool/v2ray_simple/proxy/vless"
	"github.com/hahahrfool/v2ray_simple/utils"
	"github.com/manifoldco/promptui"
)

func init() {
	cliCmdList = append(cliCmdList, CliCmd{
		"交互生成配置，超级强大", func() {
			generateConfigFileInteractively()
		},
	})
}

type CliCmd struct {
	Name string
	F    func()
}

func (cc CliCmd) String() string {
	return cc.Name
}

var cliCmdList []CliCmd

//交互式命令行用户界面
//
//阻塞，可按ctrl+C推出
func runCli() {
	defer func() {
		fmt.Printf("Interactive Mode exited. \n")
		if ce := utils.CanLogInfo("Interactive Mode exited"); ce != nil {
			ce.Write()
		}
	}()

	langList := []string{"简体中文", "English"}
	fmt.Printf("Welcome to Interactive Mode, please choose a Language \n")
	Select := promptui.Select{
		Label: "Select Language",
		Items: langList,
	}

	_, result, err := Select.Run()

	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return
	}

	fmt.Printf("You choose %q\n", result)

	if result != langList[0] {
		fmt.Printf("Sorry, language not supported yet \n")
		return
	}

	for {
		Select = promptui.Select{
			Label: "请选择想执行的功能",
			Items: cliCmdList,
		}

		i, result, err := Select.Run()

		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			return
		}

		fmt.Printf("你选择了 %q\n", result)

		if f := cliCmdList[i].F; f != nil {
			f()
		}
	}

}

func generateConfigFileInteractively() {

	rootLevelList := []string{
		"打印当前缓存的配置",
		"开始交互生成配置",
		"清除此次缓存的配置",
		"将该缓存的配置写到文件(client.toml和 server.toml)",
		"以该缓存的配置【生成客户端分享链接url】",
		"将此次生成的配置投入运行（热加载）",
	}

	confClient := proxy.Standard{}
	confServer := proxy.Standard{}

	var clientStr, serverStr string

	for {
		Select := promptui.Select{
			Label: "请选择想为你的配置文件做的事情",
			Items: rootLevelList,
		}

		i, result, err := Select.Run()

		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			return
		}

		fmt.Printf("你选择了 %q\n", result)

		generateConfStr := func() {

			confClient.Route = []*proxy.RuleConf{{
				DialTag: "direct",
				Domains: []string{"geosite:cn"},
			}}

			confClient.App = &proxy.AppConf{MyCountryISO_3166: "CN"}

			clientStr, err = utils.GetPurgedTomlStr(confClient)
			if err != nil {
				log.Fatal(err)
			}

			serverStr, err = utils.GetPurgedTomlStr(confServer)
			if err != nil {
				log.Fatal(err)
			}
		}

		switch i {
		case 0: //print

			generateConfStr()

			fmt.Printf("#客户端配置\n")
			fmt.Printf(clientStr)
			fmt.Printf("\n")

			fmt.Printf("#服务端配置\n")
			fmt.Printf(serverStr)
			fmt.Printf("\n")

		case 2: //clear
			confClient = proxy.Standard{}
			confServer = proxy.Standard{}
			clientStr = ""
			serverStr = ""
		case 3: //output

			generateConfStr()

			var clientFile *os.File
			clientFile, err = os.OpenFile("client.toml", os.O_WRONLY|os.O_CREATE, 0666)
			if err != nil {
				fmt.Println("Can't create client.toml", err)
				return
			}
			clientFile.WriteString(clientStr)
			clientFile.Close()

			var serverFile *os.File
			serverFile, err = os.OpenFile("server.toml", os.O_WRONLY|os.O_CREATE, 0666)
			if err != nil {
				fmt.Println("Can't create server.toml", err)
				return
			}
			serverFile.WriteString(serverStr)
			serverFile.Close()

			fmt.Println("生成成功！请查看文件")
		case 4: //share url
			if len(confClient.Dial) > 0 {

				fmt.Println("生成的分享链接如下：")
				d := confClient.Dial[0]
				switch d.Protocol {
				case "vless":
					fmt.Println(vless.GenerateXrayShareURL(d))

				}

			} else {
				fmt.Println("请先进行配置")

			}
		case 5: //hot load
			fmt.Println("因为本次同时生成了服务端和客户端配置, 请选择要热加载的是哪一个")
			selectHot := promptui.Select{
				Label: "加载客户端配置还是服务端配置？",
				Items: []string{
					"服务端",
					"客户端",
				},
			}
			ihot, result, err := selectHot.Run()

			if err != nil {
				fmt.Printf("Prompt failed %v\n", err)
				return
			}

			fmt.Printf("你选择了 %s\n", result)

			switch ihot {
			case 0:

				//vless in
				serverEndInServer, err := proxy.NewServer(confServer.Listen[0])
				if err != nil {
					log.Fatalln("can not create serverEndInServer: ", err)
				}
				// direct out
				serverEndOutClient, err := proxy.NewClient(confServer.Dial[0])
				if err != nil {
					log.Fatalln("can not create serverEndOutClient: ", err)
				}
				listenSer(serverEndInServer, serverEndOutClient, true)

				allServers = append(allServers, serverEndInServer)
				allClients = append(allClients, serverEndOutClient)

			case 1:
				clientEndInServer, err := proxy.NewServer(confClient.Listen[0])
				if err != nil {
					log.Fatalln("can not create clientEndInServer: ", err)
				}
				clientEndOutClient, err := proxy.NewClient(confClient.Dial[0])
				if err != nil {
					log.Fatalln("can not create clientEndOutClient: ", err)
				}
				listenSer(clientEndInServer, clientEndOutClient, true)

				allServers = append(allServers, clientEndInServer)
				allClients = append(allClients, clientEndOutClient)

			}

			fmt.Printf("加载成功！你可以回退(ctrl+c)到上级来使用 【查询当前状态】来查询新增的配置\n")

		case 1: //interactively generate

			select0 := promptui.Select{
				Label: "【提醒】我们交互模式生成的配置都是直接带tls的,且客户端【默认使用utls】模拟chrome指纹",
				Items: []string{"知道了"},
			}

			_, _, err := select0.Run()
			if err != nil {
				fmt.Printf("Prompt failed %v\n", err)
				return
			}

			select2 := promptui.Select{
				Label: "请选择你客户端想监听的协议",
				Items: []string{
					"socks5",
					"http",
				},
			}
			i2, result, err := select2.Run()

			if err != nil {
				fmt.Printf("Prompt failed %v\n", err)
				return
			}

			fmt.Printf("你选择了 %q\n", result)

			if i2 < 2 {
				confClient.Listen = append(confClient.Listen, &proxy.ListenConf{})
			} else {
				fmt.Printf("Prompt failed, werid input")
				return
			}

			clientlisten := confClient.Listen[0]
			clientlisten.Protocol = result
			clientlisten.Tag = "my_" + result

			var theInt int64

			var canLowPort bool
			validatePort := func(input string) error {
				theInt, err = strconv.ParseInt(input, 10, 64)
				if err != nil {
					return errors.New("Invalid number")
				}
				if !canLowPort {
					if theInt <= 1024 {
						return errors.New("Invalid number")
					}
				}
				if theInt > 65535 {
					return errors.New("Invalid number")
				}
				return nil
			}

			fmt.Printf("请输入你客户端想监听的端口\n")

			promptPort := promptui.Prompt{
				Label:    "Port Number",
				Validate: validatePort,
			}

			result, err = promptPort.Run()

			if err != nil {
				fmt.Printf("Prompt failed %v\n", err)
				return
			}

			fmt.Printf("你输入了 %d\n", theInt)

			clientlisten.Port = int(theInt)
			clientlisten.IP = "127.0.0.1"

			select3 := promptui.Select{
				Label: "请选择你客户端想拨号的协议(与服务端监听协议相同)",
				Items: []string{
					"vless",
				},
			}
			i3, result, err := select3.Run()

			if err != nil || i3 != 0 {
				fmt.Println("Prompt failed ", err, i3)
				return
			}

			fmt.Printf("你选择了 %q\n", result)

			confClient.Dial = append(confClient.Dial, &proxy.DialConf{})
			clientDial := confClient.Dial[0]

			fmt.Printf("请输入你服务端想监听的端口\n")
			canLowPort = true

			result, err = promptPort.Run()

			if err != nil {
				fmt.Printf("Prompt failed %v\n", err)
				return
			}

			fmt.Printf("你输入了 %d\n", theInt)

			clientDial.Port = int(theInt)
			clientDial.Protocol = "vless"
			clientDial.TLS = true
			clientDial.Tag = "my_vless"
			clientDial.Utls = true

			select4 := promptui.Select{
				Label: "请选择你客户端拨号想使用的高级层(与服务端监听的高级层相同)",
				Items: []string{
					"无",
					"ws",
					"grpc",
					"quic",
				},
			}
			i4, result, err := select4.Run()

			if err != nil {
				fmt.Println("Prompt failed ", err, i3)
				return
			}

			switch i4 {
			case 0:
			default:
				clientDial.AdvancedLayer = result
				switch i4 {
				case 1, 2:
					clientlisten.Tag += "_" + result
					promptPath := promptui.Prompt{
						Label: "Path",
						Validate: func(s string) error {
							if result == "ws" && !strings.HasPrefix(s, "/") {
								return errors.New("ws path must start with /")
							}
							return nil
						},
					}

					result, err = promptPath.Run()
					if err != nil {
						fmt.Println("Prompt failed ", err, result)
						return
					}

					fmt.Printf("你输入了 %s\n", result)

					clientDial.Path = result

				}
			}

			fmt.Printf("请输入你服务端的ip\n")

			promptIP := promptui.Prompt{
				Label:    "IP",
				Validate: utils.WrapFuncForPromptUI(govalidator.IsIP),
			}

			result, err = promptIP.Run()
			if err != nil {
				fmt.Println("Prompt failed ", err, result)
				return
			}

			fmt.Printf("你输入了 %s\n", result)

			clientDial.IP = result

			fmt.Printf("请输入你服务端的域名\n")

			promptDomain := promptui.Prompt{
				Label:    "域名",
				Validate: utils.WrapFuncForPromptUI(govalidator.IsDNSName),
			}

			result, err = promptDomain.Run()
			if err != nil {
				fmt.Println("Prompt failed ", err, result)
				return
			}

			fmt.Printf("你输入了 %s\n", result)

			clientDial.Host = result

			select5 := promptui.Select{
				Label: "请选择uuid生成方式",
				Items: []string{
					"随机",
					"手动输入(要保证你输入的是格式正确的uuid)",
				},
			}
			i5, result, err := select5.Run()

			if err != nil {
				fmt.Println("Prompt failed ", err, i3)
				return
			}
			if i5 == 0 {
				uuid := utils.GenerateUUIDStr()
				clientDial.Uuid = uuid
				fmt.Println("随机生成的uuid为", uuid)
			} else {
				promptUUID := promptui.Prompt{
					Label:    "uuid",
					Validate: utils.WrapFuncForPromptUI(govalidator.IsUUID),
				}

				result, err = promptUUID.Run()
				if err != nil {
					fmt.Println("Prompt failed ", err, result)
					return
				}

				fmt.Printf("你输入了 %s\n", result)

				clientDial.Uuid = result
			}

			var serverListenStruct proxy.ListenConf
			serverListenStruct.CommonConf = clientDial.CommonConf

			confServer.Listen = append(confServer.Listen, &serverListenStruct)

			confServer.Dial = append(confServer.Dial, &proxy.DialConf{
				CommonConf: proxy.CommonConf{
					Protocol: "direct",
				},
			})

			serverListen := confServer.Listen[0]

			select6 := promptui.Select{
				Label: "请配置服务端tls证书路径",
				Items: []string{
					"默认(cert.pem和cert.key),此时将自动开启 insecure",
					"手动输入(要保证你输入的是正确的文件路径)",
				},
			}
			i6, result, err := select6.Run()

			if err != nil {
				fmt.Println("Prompt failed ", err, i3)
				return
			}
			if i6 == 0 {
				serverListen.TLSCert = "cert.pem"
				serverListen.TLSKey = "cert.key"
				serverListen.Insecure = true
				clientDial.Insecure = true
			} else {
				fmt.Printf("请输入 cert路径\n")

				promptCPath := promptui.Prompt{
					Label:    "path",
					Validate: utils.IsFilePath,
				}

				result, err = promptCPath.Run()
				if err != nil {
					fmt.Println("Prompt failed ", err, result)
					return
				}

				fmt.Printf("你输入了 %s\n", result)

				serverListen.TLSCert = result

				fmt.Printf("请输入 key 路径\n")

				result, err = promptCPath.Run()
				if err != nil {
					fmt.Println("Prompt failed ", err, result)
					return
				}

				fmt.Printf("你输入了 %s\n", result)

				serverListen.TLSKey = result
			}

		} // switch i case 1
	} //for
}
