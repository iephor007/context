package chat

import (
	"contexts/ctx"
	"contexts/web"
	"toolkit"
)

func check(m *ctx.Message, arg []string) ([]string, string, bool) {
	if !m.Options("sessid") || !m.Options("username") {
		return nil, "", false
	}

	rid := m.Option("river")
	if len(arg[0]) != 32 {
		arg[0] = m.Cmdx("aaa.short", arg[0])
	}
	if m.Confs("flow", arg[0]) {
		rid, arg = arg[0], arg[1:]
	}
	if rid != "" && len(rid) != 32 {
		rid = m.Cmdx("aaa.short", rid)
	}
	return arg, rid, true
}

var Index = &ctx.Context{Name: "chat", Help: "会议中心",
	Caches: map[string]*ctx.Cache{},
	Configs: map[string]*ctx.Config{
		"login": &ctx.Config{Name: "login", Value: map[string]interface{}{"check": false, "local": true}, Help: "默认组件"},
		"componet": &ctx.Config{Name: "componet", Value: map[string]interface{}{
			"index": []interface{}{
				map[string]interface{}{"name": "chat",
					"tmpl": "head", "metas": []interface{}{map[string]interface{}{
						"name": "viewport", "content": "width=device-width, initial-scale=0.7, user-scalable=no",
					}}, "favicon": "favicon.ico", "styles": []interface{}{
						"example.css", "chat.css",
					}},
				map[string]interface{}{"name": "login",
					"tmpl": "fieldset", "view": "Login dialog", "init": "initLogin",
					"ctx": "web.chat", "cmd": "login",
				},
				map[string]interface{}{"name": "header",
					"tmpl": "fieldset", "view": "Header", "init": "initHeader",
					"ctx": "web.chat", "cmd": "login",
				},

				map[string]interface{}{"name": "ocean",
					"tmpl": "fieldset", "view": "Ocean dialog", "init": "initOcean",
					"ctx": "web.chat", "cmd": "ocean",
				},
				map[string]interface{}{"name": "steam",
					"tmpl": "fieldset", "view": "Steam dialog", "init": "initSteam",
					"ctx": "web.chat", "cmd": "steam",
				},
				map[string]interface{}{"name": "river",
					"tmpl": "fieldset", "view": "River", "init": "initRiver",
					"ctx": "web.chat", "cmd": "river",
				},
				map[string]interface{}{"name": "storm",
					"tmpl": "fieldset", "view": "Storm", "init": "initStorm",
					"ctx": "web.chat", "cmd": "storm",
				},

				map[string]interface{}{"name": "target",
					"tmpl": "fieldset", "view": "Target", "init": "initTarget",
					"ctx": "web.chat", "cmd": "river",
				},
				map[string]interface{}{"name": "source",
					"tmpl": "fieldset", "view": "Source", "init": "initSource",
					"ctx": "web.chat", "cmd": "storm",
				},
				map[string]interface{}{"name": "action",
					"tmpl": "fieldset", "view": "Action", "init": "initAction",
					"ctx": "web.chat", "cmd": "storm",
				},

				map[string]interface{}{"name": "footer",
					"tmpl": "fieldset", "view": "Footer", "init": "initFooter",
					"title": "shycontext 地势坤，君子以厚德载物",
				},
				map[string]interface{}{"name": "tail",
					"tmpl": "tail", "scripts": []interface{}{
						"toolkit.js", "context.js", "example.js", "chat.js",
					},
				},
			},
		}, Help: "组件列表"},
	},
	Commands: map[string]*ctx.Command{
		"login": &ctx.Command{Name: "login [username password]", Help: "登录", Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) (e error) {
			// 非登录态
			if !m.Options("sessid") || !m.Options("username") {
				if len(arg) > 0 {
					// 用户登录
					if m.Cmds("ssh.work", "share", arg[0]) {
						if m.Cmds("aaa.auth", "username", arg[0], "password", arg[1]) {
							m.Option("username", arg[0])
							m.Option("sessid", m.Cmdx("aaa.user", "session", "select"))
							if !m.Cmds("aaa.auth", "username", arg[0], "data", "chat.default") && m.Option("username") != m.Conf("runtime", "work.name") {
								m.Cmd("aaa.auth", "username", arg[0], "data", "chat.default",
									m.Cmdx(".ocean", "spawn", "", m.Option("username")+"@"+m.Conf("runtime", "work.name"), m.Option("username")))
							}
							m.Echo(m.Option("sessid"))
						}
					}
				}
				return
			}

			if len(arg) > 0 {
				switch arg[0] {
				case "share":
					m.Append("qrcode", arg[1])
					return
				case "relay":
					relay := m.Cmdx("aaa.relay", "share", arg[1:])
					m.Log("info", "relay: %s", relay)
					m.Echo(m.Cmdx("aaa.short", relay))
					return
				case "rename":
					m.Cmd("aaa.auth", "username", m.Option("username"), "data", "nickname", arg[1])

				case "weixin":
				}
			}

			// if m.Log("info", "nickname: %s", m.Option("nickname", m.Cmdx("aaa.auth", "username", m.Option("username"), "data", "nickname"))); !m.Options("nickname") {
			//	m.Option("nickname", m.Option("username"))
			// }
			m.Append("remote_ip", m.Option("remote_ip"))
			m.Append("nickname", kit.Select(m.Option("username"), m.Option("nickname")))
			m.Echo(m.Option("username"))
			return
		}},
		"ocean": &ctx.Command{Name: "ocean [search [name]]|[spawn hash name user...]", Help: "海洋, search [name]: 搜索员工, spawn hash name user...: 创建群聊", Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) (e error) {
			// 登录失败
			if !m.Options("sessid") || !m.Options("username") {
				return
			}

			if len(arg) == 0 {
				arg = append(arg, "search")
			}

			switch arg[0] {
			// 搜索员工
			case "search":
				m.Cmdy("ssh.work", "search")

			// 创建群聊
			case "spawn":
				// 用户列表
				user := map[string]interface{}{}
				if len(arg) > 3 {
					arg = append(arg, m.Option("username"))
					arg = append(arg, m.Conf("runtime", "work.name"))
					for _, v := range arg[3:] {
						if p := m.Cmdx("ssh._route", m.Conf("runtime", "work.route"), "_check", "work", v); p != "" {
							user[v] = map[string]interface{}{"user": p}
						}
					}
				}

				// 添加群聊
				h := kit.Select(kit.Hashs("uniq"), arg, 1)
				m.Conf("flow", h, map[string]interface{}{
					"conf": map[string]interface{}{
						"create_user": m.Option("username"),
						"create_time": m.Time(),
						"update_time": m.Time(),
						"nick":        kit.Select("what", arg, 2),
						"route":       kit.Select(m.Conf("runtime", "node.route"), m.Option("node.route"), arg[1] != ""),
					},
					"user": user,
					"tool": map[string]interface{}{},
					"text": map[string]interface{}{},
				})

				if m.Echo(h); arg[1] != "" {
					return
				}

				m.Cmdx(".steam", h, "spawn", "index")

				// 分发群聊
				m.Confm("flow", []string{h, "user"}, func(key string, value map[string]interface{}) {
					if kit.Right(value["user"]) && kit.Format(value["user"]) != m.Conf("runtime", "node.route") {
						m.Cmd("ssh._route", value["user"], "context", "chat", "ocean", "spawn", h, arg[2])
					}
				})
			}
			return
		}},
		"river": &ctx.Command{Name: "river hash [brow begin]|[flow type text [index]]|[wave route group index args...]", Help: "河流", Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) (e error) {
			// 登录失败
			if !m.Options("sessid") || !m.Options("username") {
				return
			}
			m.Short("river")

			// 自动入群
			if m.Options("river") {
				if m.Confs("flow", m.Option("river")) && !m.Confs("flow", []string{m.Option("river"), "user", m.Option("username")}) {
					u := m.Cmdx("ssh._route", m.Conf("runtime", "work.route"), "_check", "work", m.Option("username"))
					m.Conf("flow", []string{m.Option("river"), "user", m.Option("username"), "user"}, u)
				}
			}

			// 群聊列表
			if len(arg) == 0 {
				m.Confm("flow", func(key string, value map[string]interface{}) {
					if kit.Chain(value, []string{"user", m.Option("username")}) == nil {
						return
					}

					m.Push("key", m.Cmdx("aaa.short", key))
					m.Push("nick", kit.Chains(value, "conf.nick"))
					m.Push("create_user", kit.Chains(value, "conf.create_user"))
					m.Push("create_time", kit.Chains(value, "conf.create_time"))
					m.Push("update_time", kit.Chains(value, "conf.update_time"))

					if list, ok := kit.Chain(value, "text.list").([]interface{}); ok {
						m.Push("count", len(list))
					} else {
						m.Push("count", 0)
					}
				})
				if !m.Appends("key") {
					m.Cmd(".ocean", "spawn", "", "hello", m.Option("username"))
					m.Cmdy(".river")
					return
				}
				m.Sort("name").Sort("update_time", "time_r").Table()
				return
			}

			// 登录失败
			arg, rid, ok := check(m, arg)
			if !ok {
				return
			}

			switch arg[0] {
			// 消息列表
			case "brow":
				begin := kit.Int(kit.Select("0", arg, 1))
				m.Confm("flow", []string{rid, "text.list"}, func(index int, value map[string]interface{}) {
					if index < begin {
						return
					}
					m.Push("index", index)
					m.Push("type", value["type"])
					m.Push("text", value["text"])
					m.Push("create_time", value["create_time"])
					m.Push("create_user", value["create_user"])
					m.Push("create_nick", value["create_nick"])
				})
				m.Table()
				return

			// 推送消息
			case "flow":
				up := m.Conf("flow", []string{rid, "conf.route"})

				// 上传消息
				if len(arg) == 3 && up != m.Conf("runtime", "node.route") {
					m.Cmdy("ssh._route", up, "context", "chat", "river", rid, "flow", arg[1], arg[2])
					return
				}

				// 保存消息
				m.Conf("flow", []string{rid, "text.list.-2"}, map[string]interface{}{
					"create_nick": m.Option("nickname"),
					"create_user": m.Option("username"),
					"create_time": m.Time(),
					"type":        arg[1],
					"text":        arg[2],
				})
				m.Conf("flow", []string{rid, "conf.update_time"}, m.Time())
				count := m.Confi("flow", []string{rid, "text.count"}) + 1
				m.Confi("flow", []string{rid, "text.count"}, count)

				m.Append("create_user", m.Option("username"))
				m.Echo("%d", count)

				// 分发消息
				if up == m.Conf("runtime", "node.route") {
					m.Confm("flow", []string{rid, "user"}, func(key string, value map[string]interface{}) {
						if kit.Right(value["user"]) && kit.Format(value["user"]) != m.Conf("runtime", "node.route") {
							m.Cmd("ssh._route", value["user"], "context", "chat", "river", rid, "flow", arg[1], arg[2], count, "sync")
						}
					})
				}

			// 推送命令
			case "wave":
				m.Cmdy("ssh._route", arg[1], "tool", "run", arg[2], arg[3], rid, arg[4:])
			}
			return
		}},
		"storm": &ctx.Command{Name: "storm [rid] [[delete] group [index [arg...]]]", Help: "风雨", Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) (e error) {
			// 登录失败
			arg, rid, ok := check(m, arg)
			if !ok {
				return
			}

			// 命令列表
			if len(arg) == 0 {
				m.Confm("flow", []string{rid, "tool"}, func(key string, value map[string]interface{}) {
					m.Push("key", key)
					m.Push("count", len(value["list"].([]interface{})))
				})
				m.Sort("key").Table()
				return
			}

			switch arg[0] {
			// 删除命令
			case "delete":
				str := m.Conf("flow", []string{rid, "tool", arg[1]})
				m.Log("info", "delete %v %v %v", rid, arg[1], str)
				m.Conf("flow", []string{rid, "tool", arg[1]}, "")
				m.Echo(str)

			default:
				// 命令列表
				m.Set("option", "name")
				m.Set("option", "init")
				m.Set("option", "view")
				if len(arg) == 1 {
					short := m.Cmdx("aaa.short", rid)
					m.Confm("flow", []string{rid, "tool", arg[0], "list"}, func(index int, tool map[string]interface{}) {
						m.Push("river", short)
						m.Push("storm", arg[0])
						m.Push("action", index)

						m.Push("node", tool["node"])
						m.Push("group", tool["group"])
						m.Push("index", tool["index"])

						msg := m.Cmd("ssh._route", tool["node"], "tool", tool["group"], tool["index"])

						m.Push("name", msg.Append("name"))
						m.Push("help", msg.Append("help"))
						m.Push("view", msg.Append("view"))
						m.Push("init", msg.Append("init"))
						m.Push("inputs", msg.Append("inputs"))
						m.Push("exports", msg.Append("exports"))
						m.Push("display", msg.Append("display"))
						m.Push("feature", msg.Append("feature"))
					})
					m.Table()
					break
				}

				// 推送命令
				if tool := m.Confm("flow", []string{rid, "tool", arg[0], "list", arg[1]}); tool != nil {
					m.Cmdy("ssh._route", tool["node"], "tool", "run", tool["group"], tool["index"], rid, arg[2:])
				}
			}

			return
		}},
		"steam": &ctx.Command{Name: "steam rid [user node]|[spawn name [route group index name]...]", Help: "天空", Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) (e error) {
			// 登录失败
			arg, rid, ok := check(m, arg)
			if !ok {
				return
			}

			// 上传请求
			if up := m.Conf("flow", []string{rid, "conf.route"}); up != m.Conf("runtime", "node.route") {
				m.Cmdy("ssh._remote", up, "context", "chat", "steam", rid, arg)
				return
			}

			// 设备列表
			if len(arg) == 0 {
				m.Confm("flow", []string{rid, "user"}, func(key string, value map[string]interface{}) {
					m.Push("user", key)
					m.Push("node", value["user"])
				})
				m.Confm("ssh.node", func(key string, value map[string]interface{}) {
					m.Push("user", "")
					m.Push("node", value["name"])
				})
				m.Table()
				return
			}

			switch arg[0] {
			// 创建命令
			case "spawn":
				if len(arg) == 2 {
					self := m.Conf("runtime", "node.route")
					m.Confm("ssh.componet", arg[1], func(index int, value map[string]interface{}) {
						arg = append(arg, self, arg[1], kit.Format(index), kit.Format(value["name"]))
					})
				}

				list := []interface{}{}
				for i := 2; i < len(arg)-3; i += 4 {
					list = append(list, map[string]interface{}{
						"node": arg[i], "group": arg[i+1], "index": arg[i+2], "name": arg[i+3],
					})
				}

				m.Conf("flow", []string{rid, "tool", arg[1]}, map[string]interface{}{
					"create_user": m.Option("username"),
					"create_time": m.Time(),
					"list":        list,
				})

			// 命令列表
			default:
				m.Cmdy("ssh._route", arg[1], "tool")
			}
			return
		}},
	},
}

func init() {
	web.Index.Register(Index, &web.WEB{Context: Index})
}
