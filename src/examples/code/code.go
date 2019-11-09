package code

import (
	"contexts/ctx"
	"contexts/web"
	"fmt"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"time"
	"toolkit"
)

var Dockfile = `
FROM {{options . "base"}}

WORKDIR /home/{{options . "user"}}/context
Env ctx_dev {{options . "host"}}

RUN wget -q -O - $ctx_dev/publish/boot.sh | sh -s install

CMD sh bin/boot.sh

`

var Index = &ctx.Context{Name: "code", Help: "代码中心",
	Caches: map[string]*ctx.Cache{},
	Configs: map[string]*ctx.Config{
		"login": {Name: "login", Value: map[string]interface{}{"check": false, "local": true, "expire": "720h"}, Help: "用户登录"},
		"prefix": {Name: "prefix", Help: "外部命令", Value: map[string]interface{}{
			"zsh":    []interface{}{"cli.system", "zsh"},
			"tmux":   []interface{}{"cli.system", "tmux"},
			"docker": []interface{}{"cli.system", "docker"},
			"git":    []interface{}{"cli.system", "git"},
			"vim":    []interface{}{"cli.system", "vim"},
		}},
		"package": {Name: "package", Help: "软件包", Value: map[string]interface{}{
			"udpate":  []interface{}{"apk", "update"},
			"install": []interface{}{"apk", "add"},
			"build":   []interface{}{"build-base"},
			"develop": []interface{}{"zsh", "tmux", "git", "vim", "golang"},
			"product": []interface{}{"nginx", "redis", "mysql"},
		}},
		"docker": {Name: "docker", Help: "容器", Value: map[string]interface{}{
			"shy": Dockfile,
		}},
		"git": {Name: "git", Help: "记录", Value: map[string]interface{}{
			"alias": map[string]interface{}{"s": "status", "b": "branch"},
		}},
		"vim": {Name: "vim", Help: "记录", Value: map[string]interface{}{
			"editor": map[string]interface{}{},
			"opens":  map[string]interface{}{},
		}},
		"zsh": {Name: "vim", Help: "记录", Value: map[string]interface{}{
			"terminal": map[string]interface{}{},
			"history":  map[string]interface{}{},
		}},
		"cache": {Name: "flow", Help: "记录", Value: map[string]interface{}{
			"store": "hi.csv",
			"limit": 6,
			"least": 3,
		}},
	},
	Commands: map[string]*ctx.Command{
		"/zsh": {Name: "/zsh", Help: "终端", Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) (e error) {
			if !m.Has("res") {
				switch m.Option("cmd") {
				case "login":
					name := kit.Hashs(m.Option("pid"), m.Option("hostname"), m.Option("username"))
					m.Conf("zsh", []string{"terminal", name}, map[string]interface{}{
						"sid":      name,
						"status":   "login",
						"time":     m.Time(),
						"pwd":      m.Option("pwd"),
						"pid":      m.Option("pid"),
						"pane":     m.Option("pane"),
						"hostname": m.Option("hostname"),
						"username": m.Option("username"),
					})
					m.Echo(name)
					return
				case "logout":
					name := m.Option("sid")
					m.Conf("zsh", []string{"terminal", name, "status"}, "logout")
					m.Conf("zsh", []string{"terminal", name, "time"}, m.Time())
					return

				case "historys":
					m.Option("cache.store", "hi.csv")
					name := m.Option("sid")
					vs := strings.SplitN(strings.TrimSpace(m.Option("arg")), " ", 2)
					m.Grow("zsh", []string{"history", name}, map[string]interface{}{
						"sid":   name,
						"time":  m.Time(),
						"index": vs[0],
						"cmd":   kit.Select("", vs, 1),
						"pwd":   m.Option("pwd"),
					})

				case "history":
					m.Option("cache.store", "hi.csv")
					switch path.Base(m.Option("SHELL")) {
					case "zsh":
						m.Option("arg", strings.SplitN(m.Option("arg"), ";", 2)[1])
					}

					name := m.Option("sid")
					m.Grow("zsh", []string{"history", name}, map[string]interface{}{
						"sid":  name,
						"time": m.Time(),
						"cmd":  m.Option("arg"),
						"pwd":  m.Option("pwd"),
					})
					return
				}
			}
			return
		}},
		"zsh": {Name: "zsh dir grep key [split reg fields] [filter reg fields] [order key method] [group keys method] [sort keys method]",
			Form: map[string]int{"split": 2, "filter": 2, "order": 2, "group": 2, "sort": 2, "limit": 2},
			Help: "终端", Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) (e error) {
				p, arg := kit.Select(".", arg[0]), arg[1:]
				switch arg[0] {
				case "prune":
					ps := []string{}
					m.Confm("zsh", []string{"terminal"}, func(key string, value map[string]interface{}) {
						if kit.Format(value["status"]) == "logout" {
							ps = append(ps, key)
						}
					})
					for _, v := range ps {
						m.Log("info", "prune zsh %v %v", v, kit.Formats(m.Conf("zsh", []string{"terminal", v})))
						m.Confv("zsh", []string{"terminal", v}, "")
					}
					fallthrough
				case "terminal":
					m.Confm("zsh", "terminal", func(key string, value map[string]interface{}) {
						m.Push([]string{"time", "sid", "status", "pwd", "pid", "pane", "hostname", "username"}, value)
					})
					m.Sort("time", "time_r").Table()
				case "history":
					m.Confm("zsh", "history", func(key string, meta map[string]interface{}, index int, value map[string]interface{}) {
						if len(arg) > 1 && !strings.HasPrefix(key, arg[1]) {
							return
						}
						if sid := kit.Format(value["sid"]); len(sid) > 6 {
							value["sid"] = sid[:6]
						}
						m.Push([]string{"time", "sid", "index", "cmd", "pwd"}, value)
					})
					if len(arg) > 1 {
						m.Sort("index", "int_r").Table()
					} else {
						m.Sort("time", "time_r").Table()
					}

				case "init":
					m.Cmd("cli.system", m.Confv("package", "upadte"))
					for _, v := range kit.View(arg[1:], m.Confm("package")) {
						m.Cmd("cli.system", m.Confv("package", "install"), v)
					}

				case "list":
					m.Cmdy("nfs.dir", p, "time", "size", "path").Sort("time", "time_r").Table()

				case "find":
					m.Cmdy("cli.system", "find", p, "-name", arg[1], "cmd_parse", "cut", "", "1", "path")

				case "tail":
					m.Cmdy("cli.system", "tail", path.Join(p, arg[1]))

				case "grep":
					s, _ := os.Stat(p)
					prefix := []string{"cli.system", "grep", "-rn", arg[1], p, "cmd_parse", "cut", ":", kit.Select("2", "3", s.IsDir()), kit.Select("line text", "path line text", s.IsDir())}
					if m.Options("split") {
						re, _ := regexp.Compile(kit.Select("", m.Optionv("split"), 0))
						fields := map[string]bool{}
						for _, v := range strings.Split(kit.Select("", m.Optionv("split"), 1), " ") {
							if v != "" {
								fields[v] = true
							}
						}

						m.Cmd(prefix).Table(func(index int, line map[string]string) {
							if ls := re.FindAllStringSubmatch(line["text"], -1); len(ls) > 0 {
								m.Push("path", line["path"])
								m.Push("line", line["line"])
								for _, v := range ls {
									if len(fields) == 0 || fields[v[1]] {
										m.Push(v[1], v[2])
									}
								}
							}
						})
						m.Table()
					} else {
						m.Cmdy(prefix)
					}

					if m.Has("filter") {
						m.Filter(m.Option("filter"))
					}
					if m.Has("order") {
						m.Sort(kit.Select("", m.Optionv("order"), 0), kit.Select("", m.Optionv("order"), 1))
					}
					if m.Has("group") {
						m.Group(kit.Select("sum", m.Optionv("group"), 1), strings.Split(kit.Select("", m.Option("group"), 0), " ")...)
					}
					if m.Has("sort") {
						m.Sort(kit.Select("", m.Optionv("sort"), 0), kit.Select("", m.Optionv("sort"), 1))
					}
					if m.Has("limit") {
						m.Limit(kit.Int(kit.Select("0", m.Optionv("limit"), 0)), kit.Int(kit.Select("10", m.Optionv("limit"), 1)))
					}

				default:
					m.Cmdy("cli.system", arg)
				}
				return
			}},
		"tmux": {Name: "tmux [session [window [pane cmd]]]", Help: "窗口", Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) (e error) {
			prefix := kit.Trans(m.Confv("prefix", "tmux"))
			// 修改信息
			if len(arg) > 1 {
				switch arg[1] {
				case "modify":
					switch arg[2] {
					case "session":
						m.Cmdy(prefix, "rename-session", "-t", arg[0], arg[3])
					case "window":
						m.Cmdy(prefix, "rename-window", "-t", arg[0], arg[3])
					}
					return
				}
			}

			// 查看会话
			if m.Cmdy(prefix, "list-session", "-F", "#{session_id},#{session_name},#{session_windows},#{session_height},#{session_width}",
				"cmd_parse", "cut", ",", "5", "id session windows height width"); len(arg) == 0 {
				return
			}

			// 创建会话
			if arg[0] != "" && !kit.Contains(m.Meta["session"], arg[0]) {
				m.Cmdy(prefix, "new-session", "-ds", arg[0])
			}
			m.Set("append").Set("result")

			// 查看窗口
			if m.Cmdy(prefix, "list-windows", "-t", arg[0], "-F", "#{window_id},#{window_name},#{window_panes},#{window_height},#{window_width}",
				"cmd_parse", "cut", ",", "5", "id window panes height width"); len(arg) == 1 {
				return
			}

			// 创建窗口
			if arg[1] != "" && !kit.Contains(m.Meta["window"], arg[1]) {
				m.Cmdy(prefix, "new-window", "-dt", arg[0], "-n", arg[1])
			}
			m.Set("append").Set("result")

			// 查看面板
			if len(arg) == 2 {
				m.Cmdy(prefix, "list-panes", "-t", arg[0]+":"+arg[1], "-F", "#{pane_id},#{pane_index},#{pane_tty},#{pane_height},#{pane_width}",
					"cmd_parse", "cut", ",", "5", "id pane tty height width")
				return
			}

			// 执行命令
			target := arg[0] + ":" + arg[1] + "." + arg[2]
			if len(arg) > 3 {
				// 修改缓存
				if len(arg) > 5 {
					switch arg[5] {
					case "modify":
						switch arg[6] {
						case "text":
							m.Cmdy(prefix, "set-buffer", "-b", arg[4], arg[7])
						}
						return
					}
				}

				switch arg[3] {
				// 操作缓存
				case "buffer":
					if len(arg) > 5 {
						m.Cmdy(prefix, "set-buffer", "-b", arg[4], arg[5])
					}
					if len(arg) > 4 {
						m.Cmdy(prefix, "show-buffer", "-b", arg[4])
						return
					}
					m.Cmdy(prefix, "list-buffers", "cmd_parse", "cut", ": ", "3", "buffer size text")
					for i, v := range m.Meta["text"] {
						if i < 3 {
							m.Meta["text"][i] = m.Cmdx(prefix, "show-buffer", "-b", m.Meta["buffer"][i])
						} else {
							m.Meta["text"][i] = v[2 : len(v)-1]
						}
					}
					return
				// 面板列表
				case "pane":
					m.Cmdy(prefix, "list-panes", "-a", "cmd_parse", "cut", " ", "8", "pane_name size some lines bytes haha pane_id tag")
					m.Meta["append"] = []string{"pane_id", "pane_name", "size", "lines", "bytes", "tag"}
					m.Table(func(index int, value map[string]string) {
						m.Meta["pane_name"][index] = strings.TrimSuffix(value["pane_name"], ":")
						m.Meta["pane_id"][index] = strings.TrimPrefix(value["pane_id"], "%")
						m.Meta["lines"][index] = strings.TrimSuffix(value["lines"], ",")
						m.Meta["bytes"][index] = kit.FmtSize(kit.Int64(value["bytes"]))
					})
					m.Sort("pane_name")
					m.Table()
					return
				// 运行命令
				case "run":
					arg = arg[1:]
					fallthrough
				default:
					m.Cmdy(prefix, "send-keys", "-t", target, strings.Join(arg[3:], " "), "Enter")
					time.Sleep(1 * time.Second)
				}
			}

			// 查看终端
			m.Echo(strings.TrimSpace(m.Cmdx(prefix, "capture-pane", "-pt", target)))
			return
		}},
		"docker": {Name: "docker", Help: "容器", Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) (e error) {
			prefix := kit.Trans(m.Confv("prefix", "docker"))
			switch arg[0] {
			case "image":
				prefix = append(prefix, "image")
				pos := kit.Select("shy", arg, 1)
				tag := kit.Select("2.1", arg, 2)

				// 查看镜像
				if m.Cmdy(prefix, "ls", "cmd_parse", "cut", "cmd_headers", "IMAGE ID", "IMAGE_ID"); len(arg) == 1 {
					return
				} else if i := kit.IndexOf(m.Meta["IMAGE_ID"], arg[1]); i > -1 {
					arg, pos, tag = arg[2:], strings.TrimSpace(m.Meta["REPOSITORY"][i]), strings.TrimSpace(m.Meta["TAG"][i])
				} else {
					arg = arg[3:]
				}

				// 拉取镜像
				if len(arg) == 0 {
					m.Cmdy(prefix, "pull", pos+":"+tag)
					return
				}

				switch arg[0] {
				// 启动容器
				case "运行":
					m.Set("append").Set("result")
					m.Cmdy(prefix[:2], "run", "-dt", pos+":"+tag)
					return
				// 清理镜像
				case "清理":
					m.Cmd(prefix, "prune", "-f")

				// 删除镜像
				case "delete":
					m.Cmd(prefix, "rm", pos+":"+tag)

				// 创建镜像
				default:
					m.Option("base", pos+":"+tag)
					m.Option("name", arg[0]+":"+kit.Select("2.1", arg, 1))
					m.Option("host", "http://"+m.Conf("runtime", "boot.hostname")+".local:9095")
					m.Option("user", kit.Select("shy", arg, 2))
					m.Option("file", "etc/Dockerfile")

					if f, _, e := kit.Create(m.Option("file")); m.Assert(e) {
						defer f.Close()
						if m.Assert(ctx.ExecuteStr(m, f, m.Conf("docker", arg[0]))) {
							m.Cmdy(prefix, "build", "-f", m.Option("file"), "-t", m.Option("name"), ".")
						}
					}
				}

			case "container":
				prefix = append(prefix, "container")
				if len(arg) > 1 {
					switch arg[2] {
					case "进入":
						m.Cmdy(m.Confv("prefix", "tmux"), "new-window", "-dPF", "#{session_name}:#{window_name}.1", "docker exec -it "+arg[1]+" sh")
						return

					case "停止":
						m.Cmd(prefix, "stop", arg[1])

					case "启动":
						m.Cmd(prefix, "start", arg[1])

					case "重启":
						m.Cmd(prefix, "restart", arg[1])

					case "清理":
						m.Cmd(prefix, "prune", "-f")

					case "modify":
						switch arg[3] {
						case "NAMES":
							m.Cmd(prefix, "rename", arg[1], arg[4:])
						}

					case "delete":
						m.Cmd(prefix, "rm", arg[1])

					default:
						if len(arg) > 2 {
							m.Cmdy(prefix, "exec", arg[1], arg[2:])
						} else {
							m.Cmdy(prefix, "inspect", arg[1])
						}
						return
					}
				}
				m.Cmdy(prefix, "ls", "-a", "cmd_parse", "cut", "cmd_headers", "CONTAINER ID", "CONTAINER_ID")

			case "network":
				prefix = append(prefix, "network")
				if len(arg) == 1 {
					m.Cmdy(prefix, "ls", "cmd_parse", "cut", "cmd_headers", "NETWORK ID", "NETWORK_ID")
					break
				}

				kit.Map(kit.Chain(kit.UnMarshal(m.Cmdx(prefix, "inspect", arg[1])), "0.Containers"), "", func(key string, value map[string]interface{}) {
					m.Push("CONTAINER_ID", key[:12])
					m.Push("name", value["Name"])
					m.Push("IPv4", value["IPv4Address"])
					m.Push("IPv6", value["IPV4Address"])
					m.Push("Mac", value["MacAddress"])
				})
				m.Table()

			case "volume":
				if len(arg) == 1 {
					m.Cmdy(prefix, "volume", "ls", "cmd_parse", "cut", "cmd_headers", "VOLUME NAME", "VOLUME_NAME")
					break
				}

			default:
				m.Cmdy(prefix, arg)
			}
			return
		}},
		"git": {Name: "git init|diff|status|commit|branch|remote|pull|push|sum", Help: "版本", Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) (e error) {
			prefix, arg := append(kit.Trans(m.Confv("prefix", "git")), "cmd_dir", kit.Select(".", arg[0])), arg[1:]

			switch arg[0] {
			case "init":
				if s, e := os.Stat(path.Join(prefix[len(prefix)-1], ".git")); e == nil && s.IsDir() {
					if len(arg) > 1 {
						m.Cmdy(prefix, "remote", "add", "-f", kit.Select("origin", arg, 2), arg[1])
					}
				} else if len(arg) > 1 {
					m.Cmdy(prefix, "clone", arg[1], ".")
				} else {
					m.Cmdy(prefix, "init")
				}

				m.Confm("git", "alias", func(key string, value string) {
					m.Cmdy(prefix, "config", "alias."+key, value)
				})

			case "diff":
				m.Cmdy(prefix, "diff")
			case "status":
				m.Cmdy(prefix, "status", "-sb", "cmd_parse", "cut", " ", "2", "tags file")
			case "commit":
				if len(arg) > 1 && m.Cmdy(prefix, "commit", "-am", arg[1]).Result() == "" {
					break
				}
				m.Cmdy(prefix, "log", "--stat", "-n", "3")
			case "branch":
				if len(arg) > 1 {
					m.Cmd(prefix, "branch", arg[1])
					m.Cmd(prefix, "checkout", arg[1])
				}
				for _, v := range strings.Split(m.Cmdx(prefix, "branch", "-v"), "\n") {
					if len(v) > 0 {
						m.Push("tags", v[:2])
						vs := strings.SplitN(strings.TrimSpace(v[2:]), " ", 2)
						m.Push("branch", vs[0])
						vs = strings.SplitN(strings.TrimSpace(vs[1]), " ", 2)
						m.Push("hash", vs[0])
						m.Push("note", strings.TrimSpace(vs[1]))
					}
				}
				m.Table()
			case "remote":
				m.Cmdy(prefix, "remote", "-v", "cmd_parse", "cut", " ", "3", "remote url tag")

			case "push":
				m.Cmdy(prefix, "push")
			case "sum":
				total := false
				if len(arg) > 1 && arg[1] == "total" {
					total, arg = true, arg[1:]
				}

				args := []string{"log", "--shortstat", "--pretty=commit: %ad %n%s", "--date=iso", "--reverse"}
				if len(arg) > 1 {
					args = append(args, kit.Select("-n", "--since", strings.Contains(arg[1], "-")))
					if strings.Contains(arg[1], "-") && !strings.Contains(arg[1], ":") {
						arg[1] = arg[1] + " 00:00:00"
					}
					args = append(args, arg[1:]...)
				} else {
					args = append(args, "-n", "30")
				}

				var total_day time.Duration
				count, count_add, count_del := 0, 0, 0
				if out, e := exec.Command("git", args...).CombinedOutput(); e == nil {
					for i, v := range strings.Split(string(out), "commit: ") {
						if i > 0 {
							l := strings.Split(v, "\n")
							hs := strings.Split(l[0], " ")

							add, del := "0", "0"
							if len(l) > 3 {
								fs := strings.Split(strings.TrimSpace(l[3]), ", ")
								if adds := strings.Split(fs[1], " "); len(fs) > 2 {
									dels := strings.Split(fs[2], " ")
									add = adds[0]
									del = dels[0]
								} else if adds[1] == "insertions(+)" {
									add = adds[0]
								} else {
									del = adds[0]
								}
							}

							if total {
								if count++; i == 1 {
									if t, e := time.Parse("2006-01-02", hs[0]); e == nil {
										total_day = time.Now().Sub(t)
										m.Append("from", hs[0])
									}
								}
								count_add += kit.Int(add)
								count_del += kit.Int(del)
								continue
							}

							m.Push("date", hs[0])
							m.Push("adds", add)
							m.Push("dels", del)
							m.Push("rest", kit.Int(add)-kit.Int(del))
							m.Push("note", l[1])
							m.Push("hour", strings.Split(hs[1], ":")[0])
							m.Push("time", hs[1])
						}
					}
					if total {
						m.Append("days", int(total_day.Hours())/24)
						m.Append("commit", count)
						m.Append("adds", count_add)
						m.Append("dels", count_del)
						m.Append("rest", count_add-count_del)
					}
					m.Table()
				} else {
					m.Log("warn", "%v", string(out))
				}

			default:
				m.Cmdy(prefix, arg)
			}
			return
		}},
		"tags": {Name: "tags", Help: "代码索引", Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) (e error) {
			m.Cmdy("cli.system", "gotags", "-f", kit.Select("tags", arg, 1), "-R", kit.Select("src", arg, 0))
			return
		}},
		"vim": {Name: "vim", Help: "编辑器", Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) (e error) {
			switch arg[0] {
			case "ctag":
				if f, p, e := kit.Create("etc/conf/tags"); m.Assert(e) {
					defer f.Close()
					for k, _ := range c.Commands {
						fmt.Fprintf(f, "%s\t%s\t/\"%s\": {Name/\n", k, "../../src/examples/code/code.go", k)
					}
					m.Echo(p)
				}

			case "editor":
				m.Confm("vim", "editor", func(key string, value map[string]interface{}) {
					m.Push([]string{"time", "sid", "status", "pwd", "pid", "pane", "hostname", "username"}, value)
				})
				m.Sort("time", "time_r").Table()
			case "opens":
				m.Confm("vim", "opens", func(key string, value map[string]interface{}) {
					value["sid"] = kit.Format(value["sid"])[:6]
					m.Push([]string{"time", "sid", "action", "file"}, value)
				})
				m.Sort("time", "time_r").Table()
			case "prune":
				ps := []string{}
				m.Confm("vim", []string{"editor"}, func(key string, value map[string]interface{}) {
					if kit.Format(value["status"]) == "logout" {
						ps = append(ps, key)
					}
				})
				for _, v := range ps {
					for _, k := range []string{"editor", "exec", "tag", "fix", "buffer", "register"} {
						m.Log("info", "prune vim %v %v %v", k, v, kit.Formats(m.Conf("vim", []string{k, v})))
						m.Confv("vim", []string{k, v}, "")
					}
				}

			case "cmds":
				m.Confm("vim", "exec", func(meta map[string]interface{}, index int, value map[string]interface{}) {
					value["sid"] = kit.Format(value["sid"])[:6]
					m.Push([]string{"time", "sid", "cmd", "file", "pwd"}, value)
				})
				m.Sort("time", "time_r").Table()
			case "txts":
				m.Confm("vim", "insert", func(meta map[string]interface{}, index int, value map[string]interface{}) {
					value["sid"] = kit.Format(value["sid"])[:6]
					m.Push([]string{"time", "sid", "text", "line", "col", "file", "pwd"}, value)
				})
				m.Sort("time", "time_r").Table()
			case "tags":
				m.Confm("vim", "tag", func(key string, index int, value map[string]interface{}) {
					m.Push("sid", key[:6]).Push([]string{"tag", "line", "file"}, value)
				})
				m.Table()
			case "fixs":
				m.Confm("vim", "fix", func(key string, index int, value map[string]interface{}) {
					m.Push("sid", key[:6]).Push([]string{"fix", "file", "line", "text"}, value)
				})
				m.Table()
			case "bufs":
				m.Confm("vim", "buffer", func(key string, index int, value map[string]interface{}) {
					m.Push("sid", key[:6]).Push([]string{"buf", "tag", "file", "line"}, value)
				})
				m.Table()
			case "regs":
				m.Confm("vim", "register", func(key string, index int, value map[string]interface{}) {
					m.Push("sid", key[:6]).Push([]string{"reg", "text"}, value)
				})
				m.Table()
			case "marks":
				m.Confm("vim", "mark", func(key string, index int, value map[string]interface{}) {
					m.Push("sid", key[:6]).Push([]string{"mark", "line", "col", "file"}, value)
				})
				m.Table()
			}
			return
		}},
		"/vim": {Name: "/vim", Help: "编辑器", Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) (e error) {
			m.Option("arg", strings.Replace(m.Option("arg"), "XXXXXsingleXXXXX", "'", -1))
			m.Option("sub", strings.Replace(m.Option("sub"), "XXXXXsingleXXXXX", "'", -1))
			if !m.Has("res") {
				switch m.Option("cmd") {
				case "login":
					name := kit.Hashs(m.Option("pid"), m.Option("hostname"), m.Option("username"))
					m.Conf("vim", []string{"editor", name}, map[string]interface{}{
						"sid":      name,
						"status":   "login",
						"time":     m.Time(),
						"pwd":      m.Option("pwd"),
						"pid":      m.Option("pid"),
						"pane":     m.Option("pane"),
						"hostname": m.Option("hostname"),
						"username": m.Option("username"),
					})
					m.Echo(name)
					return
				case "logout":
					name := m.Option("sid")
					m.Conf("vim", []string{"editor", name, "status"}, "logout")
					m.Conf("vim", []string{"editor", name, "time"}, m.Time())
					return

				case "read", "write", "close":
					file := m.Option("arg")
					if !path.IsAbs(m.Option("arg")) {
						file = path.Join(m.Option("pwd"), m.Option("arg"))
					}

					name := kit.Hashs(file)
					m.Conf("vim", []string{"opens", name, "path"}, file)
					m.Conf("vim", []string{"opens", name, "time"}, m.Time())
					m.Conf("vim", []string{"opens", name, "file"}, m.Option("arg"))
					m.Conf("vim", []string{"opens", name, "action"}, m.Option("cmd"))
					m.Conf("vim", []string{"opens", name, "pwd"}, m.Option("pwd"))
					m.Conf("vim", []string{"opens", name, "sid"}, m.Option("sid"))
					return
				case "insert":
					m.Option("cache.store", "he.csv")
					m.Grow("vim", "insert", map[string]interface{}{
						"time": m.Time(),
						"sid":  m.Option("sid"),
						"text": m.Option("arg"),
						"line": m.Option("row"),
						"col":  m.Option("col"),
						"file": m.Option("buf"),
						"pwd":  m.Option("pwd"),
					})
				case "exec":
					m.Option("cache.store", "he.csv")
					m.Grow("vim", "exec", map[string]interface{}{
						"time": m.Time(),
						"sid":  m.Option("sid"),
						"cmd":  m.Option("arg"),
						"file": m.Option("buf"),
						"pwd":  m.Option("pwd"),
					})
				case "sync":
					switch m.Option("arg") {
					case "fixs":
						if m.Conf("vim", []string{"fix", m.Option("sid")}, ""); strings.HasPrefix(m.Option("sub"), "\nError") {
							break
						}
						m.Split(strings.TrimPrefix(m.Option("sub"), "\n"), " ", "3", "id file text").Table(func(index int, value map[string]string) {
							vs := strings.Split(kit.Format(value["file"]), ":")
							m.Conf("vim", []string{"fix", m.Option("sid"), "-2"}, map[string]interface{}{
								"fix":  value["id"],
								"file": vs[0],
								"line": vs[1],
								"text": value["text"],
							})
						})
						m.Set("append").Set("result")
					case "tags":
						m.Conf("vim", []string{"tag", m.Option("sid")}, "")
						m.Split(strings.TrimPrefix(m.Option("sub"), "\n"), " ", "6").Table(func(index int, value map[string]string) {
							m.Conf("vim", []string{"tag", m.Option("sid"), "-2"}, map[string]interface{}{
								"tag":  value["tag"],
								"line": value["line"],
								"file": value["in file/text"],
							})
						})
						m.Set("append").Set("result")
					case "bufs":
						m.Conf("vim", []string{"buffer", m.Option("sid")}, "")
						m.Split(strings.TrimSpace(m.Option("sub")), " ", "5", "id tag name some line").Table(func(index int, value map[string]string) {
							m.Conf("vim", []string{"buffer", m.Option("sid"), "-2"}, map[string]interface{}{
								"buf":  value["id"],
								"tag":  value["tag"],
								"file": value["name"],
								"line": value["line"],
							})
						})
						m.Set("append").Set("result")
					case "regs":
						m.Conf("vim", []string{"register", m.Option("sid")}, "")
						m.Split(strings.TrimPrefix(m.Option("sub"), "\n--- Registers ---\n"), " ", "2", "name text").Table(func(index int, value map[string]string) {
							m.Conf("vim", []string{"register", m.Option("sid"), "-2"}, map[string]interface{}{
								"text": strings.Replace(strings.Replace(value["text"], "^I", "\t", -1), "^J", "\n", -1),
								"reg":  strings.TrimPrefix(value["name"], "\""),
							})
						})
						m.Set("append").Set("result")
					case "marks":
						m.Conf("vim", []string{"mark", m.Option("sid")}, "")
						m.Split(strings.TrimPrefix(m.Option("sub"), "\n"), " ", "4").Table(func(index int, value map[string]string) {
							m.Conf("vim", []string{"mark", m.Option("sid"), "-2"}, map[string]interface{}{
								"mark": value["mark"],
								"line": value["line"],
								"col":  value["col"],
								"file": value["file/text"],
							})
						})
					}
					return
				default:
					return
				}
			}

			switch m.Option("cmd") {
			case "read", "write", "close":
			}
			return
		}},
	},
}

func init() {
	web.Index.Register(Index, &web.WEB{Context: Index})
}
