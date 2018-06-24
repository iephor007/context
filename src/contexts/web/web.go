package web // {{{
// }}}
import ( // {{{
	"contexts"
	"regexp"
	"strconv"
	"toolkit"

	"encoding/json"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"

	"bytes"
	"mime/multipart"
	"path/filepath"

	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// }}}

type MUX interface {
	Handle(string, http.Handler)
	HandleFunc(string, func(http.ResponseWriter, *http.Request))
	Trans(*ctx.Message, string, func(*ctx.Message, *ctx.Context, string, ...string))
}

type WEB struct {
	*http.ServeMux
	*http.Server

	client *http.Client
	cookie map[string]*http.Cookie

	list     map[string][]string
	list_key []string

	*ctx.Message
	*ctx.Context
}

func (web *WEB) generate(m *ctx.Message, uri string, arg ...string) string { // {{{
	add, e := url.Parse(uri)
	m.Assert(e)

	adds := []string{}

	if add.Scheme != "" {
		adds = append(adds, add.Scheme)
	} else if m.Confs("protocol") {
		adds = append(adds, m.Conf("protocol"))
	}
	adds = append(adds, "://")

	if add.Host != "" {
		adds = append(adds, add.Host)
	} else if m.Confs("hostname") {
		adds = append(adds, m.Conf("hostname"))
		if m.Confs("port") {
			adds = append(adds, ":")
			adds = append(adds, m.Conf("port"))
		}
	}

	dir, file := path.Split(add.EscapedPath())
	if path.IsAbs(dir) {
		adds = append(adds, dir)
		adds = append(adds, file)
	} else {
		adds = append(adds, m.Conf("dir"))
		if dir == "" && file == "" {
			adds = append(adds, m.Conf("file"))
		} else {
			adds = append(adds, dir)
			adds = append(adds, file)
		}
	}

	args := []string{}
	for i := 0; i < len(arg)-1; i += 2 {
		value := arg[i+1]
		if len(arg[i+1]) > 1 {
			switch arg[i+1][0] {
			case '$':
				value = m.Cap(arg[i+1][1:])
			case '@':
				value = m.Conf(arg[i+1][1:])
			}
		}

		args = append(args, arg[i]+"="+url.QueryEscape(value))
	}
	p := strings.Join(args, "&")

	if add.RawQuery != "" {
		adds = append(adds, "?")
		adds = append(adds, add.RawQuery)
		if p != "" {
			adds = append(adds, "&")
			adds = append(adds, p)
		}
	} else if m.Confs("query") {
		adds = append(adds, "?")
		adds = append(adds, m.Conf("query"))
		if p != "" {
			adds = append(adds, "&")
			adds = append(adds, p)
		}
	} else {
		if p != "" {
			adds = append(adds, "?")
			adds = append(adds, p)
		}
	}

	return strings.Join(adds, "")
}

// }}}
func (web *WEB) Trans(m *ctx.Message, key string, hand func(*ctx.Message, *ctx.Context, string, ...string)) { // {{{
	web.HandleFunc(key, func(w http.ResponseWriter, r *http.Request) {
		msg := m.Spawn().Set("detail", key)
		msg.Option("method", r.Method)
		msg.Option("referer", r.Header.Get("Referer"))

		for k, v := range r.Form {
			msg.Add("option", k, v...)
		}
		for _, v := range r.Cookies() {
			msg.Option(v.Name, v.Value)
		}

		msg.Log("cmd", nil, "%s [] %v", key, msg.Meta["option"])
		msg.Put("option", "request", r).Put("option", "response", w)

		if hand(msg, msg.Target(), key); msg.Has("redirect") {
			http.Redirect(w, r, msg.Append("redirect"), http.StatusFound)
			return
		}
		if msg.Has("template") {
			msg.Spawn().Cmd("/render", msg.Meta["template"])
			return
		}
		if msg.Has("append") {
			msg.Spawn().Copy(msg, "append").Cmd("/json")
			return
		}
		for _, v := range msg.Meta["result"] {
			w.Write([]byte(v))
		}
	})
}

// }}}
func (web *WEB) ServeHTTP(w http.ResponseWriter, r *http.Request) { // {{{
	if web.Message != nil {
		web.Log("cmd", nil, "%v %s %s", r.RemoteAddr, r.Method, r.URL)

		if web.Confs("logheaders") {
			for k, v := range r.Header {
				web.Log("info", nil, "%s: %v", k, v)
			}
			web.Log("info", nil, "")
		}

		if r.ParseForm(); len(r.PostForm) > 0 {
			for k, v := range r.PostForm {
				web.Log("info", nil, "%s: %v", k, v)
			}
			web.Log("info", nil, "")
		}
	}

	web.ServeMux.ServeHTTP(w, r)

	if web.Message != nil && web.Confs("logheaders") {
		for k, v := range w.Header() {
			web.Log("info", nil, "%s: %v", k, v)
		}
		web.Log("info", nil, "")
	}
}

// }}}

func (web *WEB) Spawn(m *ctx.Message, c *ctx.Context, arg ...string) ctx.Server { // {{{
	c.Caches = map[string]*ctx.Cache{}
	c.Configs = map[string]*ctx.Config{}

	s := new(WEB)
	s.Context = c
	return s
}

// }}}
func (web *WEB) Begin(m *ctx.Message, arg ...string) ctx.Server { // {{{
	web.Context.Master(nil)
	web.Caches["route"] = &ctx.Cache{Name: "请求路径", Value: "/" + web.Context.Name + "/", Help: "请求路径"}
	web.Caches["register"] = &ctx.Cache{Name: "已初始化(yes/no)", Value: "no", Help: "模块是否已初始化"}
	web.Caches["master"] = &ctx.Cache{Name: "服务入口(yes/no)", Value: "no", Help: "服务入口"}
	web.Caches["directory"] = &ctx.Cache{Name: "服务目录", Value: "usr", Help: "服务目录"}
	if len(arg) > 0 {
		m.Cap("directory", arg[0])
	}

	web.ServeMux = http.NewServeMux()
	if mux, ok := m.Target().Server.(MUX); ok {
		for k, x := range web.Commands {
			if k[0] == '/' {
				mux.Trans(m, k, x.Hand)
			}
		}
	}

	web.list = map[string][]string{}

	return web
}

// }}}
func (web *WEB) Start(m *ctx.Message, arg ...string) bool { // {{{
	if len(arg) > 0 {
		m.Cap("directory", arg[0])
	}

	m.Travel(m.Target(), func(m *ctx.Message) bool {
		if h, ok := m.Target().Server.(http.Handler); ok && m.Cap("register") == "no" {
			m.Cap("register", "yes")

			p, i := m.Target(), 0
			m.BackTrace(func(m *ctx.Message) bool {
				p = m.Target()
				if i++; i == 2 {
					return false
				}
				return true
			})

			if s, ok := p.Server.(MUX); ok {
				m.Log("info", p, "route %s -> %s", m.Cap("route"), m.Target().Name)
				s.Handle(m.Cap("route"), http.StripPrefix(path.Dir(m.Cap("route")), h))
			}

			if s, ok := m.Target().Server.(MUX); ok && m.Cap("directory") != "" {
				m.Log("info", nil, "dir / -> [%s]", m.Cap("directory"))
				s.Handle("/", http.FileServer(http.Dir(m.Cap("directory"))))
			}
		}
		return true
	})

	web.Caches["address"] = &ctx.Cache{Name: "服务地址", Value: ":9191", Help: "服务地址"}
	web.Caches["protocol"] = &ctx.Cache{Name: "服务协议", Value: "http", Help: "服务协议"}
	if len(arg) > 1 {
		m.Cap("address", arg[1])
	}
	if len(arg) > 2 {
		m.Cap("protocol", arg[2])
	}

	m.Cap("master", "yes")
	m.Cap("stream", m.Cap("address"))
	m.Log("info", nil, "address [%s]", m.Cap("address"))
	m.Log("info", nil, "protocol [%s]", m.Cap("protocol"))
	web.Server = &http.Server{Addr: m.Cap("address"), Handler: web}

	web.Configs["logheaders"] = &ctx.Config{Name: "日志输出报文头(yes/no)", Value: "yes", Help: "日志输出报文头"}

	if web.Message = m; m.Cap("protocol") == "https" {
		web.Caches["cert"] = &ctx.Cache{Name: "服务证书", Value: m.Conf("cert"), Help: "服务证书"}
		web.Caches["key"] = &ctx.Cache{Name: "服务密钥", Value: m.Conf("key"), Help: "服务密钥"}
		m.Log("info", nil, "cert [%s]", m.Cap("cert"))
		m.Log("info", nil, "key [%s]", m.Cap("key"))

		web.Server.ListenAndServeTLS(m.Cap("cert"), m.Cap("key"))
	} else {
		web.Server.ListenAndServe()
	}

	return true
}

// }}}
func (web *WEB) Close(m *ctx.Message, arg ...string) bool { // {{{
	switch web.Context {
	case m.Target():
	case m.Source():
	}
	return true
}

// }}}

var Index = &ctx.Context{Name: "web", Help: "应用中心",
	Caches: map[string]*ctx.Cache{
		"count": &ctx.Cache{Name: "count", Value: "0", Help: "主机协议"},
	},
	Configs: map[string]*ctx.Config{
		"protocol": &ctx.Config{Name: "protocol", Value: "", Help: "主机协议"},
		"hostname": &ctx.Config{Name: "hostname", Value: "", Help: "主机地址"},
		"port":     &ctx.Config{Name: "port", Value: "", Help: "主机端口"},
		"dir":      &ctx.Config{Name: "dir", Value: "/", Help: "主机路由"},
		"file":     &ctx.Config{Name: "file", Value: "", Help: "主机文件"},
		"query":    &ctx.Config{Name: "query", Value: "", Help: "主机参数"},
		"output":   &ctx.Config{Name: "output", Value: "stdout", Help: "响应输出"},
		"editor":   &ctx.Config{Name: "editor", Value: "vim", Help: "响应编辑器"},

		"template_dir": &ctx.Config{Name: "template_dir", Value: "usr/template/", Help: "通用模板路径"},
		"common_tmpl":  &ctx.Config{Name: "common_tmpl", Value: "common/*.html", Help: "通用模板路径"},
		"common_main":  &ctx.Config{Name: "common_main", Value: "main.html", Help: "通用模板框架"},
		"upload_tmpl":  &ctx.Config{Name: "upload_tmpl", Value: "upload.html", Help: "上传文件模板"},
		"upload_main":  &ctx.Config{Name: "upload_main", Value: "main.html", Help: "上传文件框架"},
		"travel_tmpl":  &ctx.Config{Name: "travel_tmpl", Value: "travel.html", Help: "浏览模块模板"},
		"travel_main":  &ctx.Config{Name: "travel_main", Value: "main.html", Help: "浏览模块框架"},
	},
	Commands: map[string]*ctx.Command{
		"serve": &ctx.Command{Name: "serve [directory [address [protocol]]]", Help: "开启应用服务", Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) {
			m.Set("detail", arg...).Target().Start(m)
		}},
		"route": &ctx.Command{Name: "route directory|template|script route content", Help: "添加应用内容", Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) {
			mux, ok := m.Target().Server.(MUX) // {{{
			m.Assert(ok, "模块类型错误")
			m.Assert(len(arg) == 3, "缺少参数")

			switch arg[0] {
			case "directory":
				mux.Handle(arg[1]+"/", http.StripPrefix(arg[1], http.FileServer(http.Dir(arg[2]))))
			case "template":
				mux.Trans(m, arg[1], func(m *ctx.Message, c *ctx.Context, key string, a ...string) {
					w := m.Data["response"].(http.ResponseWriter)

					if _, e := os.Stat(arg[2]); e == nil {
						template.Must(template.ParseGlob(arg[2])).Execute(w, m)
					} else {
						template.Must(template.New("temp").Parse(arg[2])).Execute(w, m)
					}

				})
			case "script":
				cli := m.Find("cli")
				lex := m.Find("lex")
				mux.Trans(m, arg[1], func(m *ctx.Message, c *ctx.Context, key string, a ...string) {
					f, e := os.Open(arg[2])
					line, bio := "", bufio.NewReader(f)
					if e != nil {
						line = arg[2]
					}

					for {
						if line = strings.TrimSpace(line); line != "" {
							lex.Cmd("split", line, "void")
							cli.Wait = make(chan bool)
							cli.Cmd(lex.Meta["result"])
							m.Meta["result"] = cli.Meta["result"]
						}

						if line, e = bio.ReadString('\n'); e != nil {
							break
						}
					}
				})
			} // }}}
		}},
		"cookie": &ctx.Command{Name: "cookie add|del arg...", Help: "访问URL", Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) {
			web, ok := m.Target().Server.(*WEB) // {{{
			m.Assert(ok)

			switch len(arg) {
			case 0:
				for k, v := range web.cookie {
					m.Echo("%s: %v\n", k, v.Value)
				}
			case 1:
				if v, ok := web.cookie[arg[0]]; ok {
					m.Echo("%s", v.Value)
				}
			default:
				if web.cookie == nil {
					web.cookie = make(map[string]*http.Cookie)
				}
				if v, ok := web.cookie[arg[0]]; ok {
					v.Value = arg[1]
				} else {
					web.cookie[arg[0]] = &http.Cookie{Name: arg[0], Value: arg[1]}
				}
			}
			// }}}
		}},
		"get": &ctx.Command{Name: "get [method GET|POST] [file filename] url arg...", Help: "访问URL",
			Formats: map[string]int{"method": 1, "file": 2},
			Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) {
				web, ok := m.Target().Server.(*WEB) // {{{
				m.Assert(ok)

				if web.client == nil {
					web.client = &http.Client{}
				}

				method := "GET"
				if m.Options("method") {
					method = m.Option("method")
				}

				uri := web.generate(m, arg[0], arg[1:]...)
				m.Log("info", nil, "GET %s", uri)
				m.Echo("%s: %s\n", method, uri)

				var body io.Reader
				index := strings.Index(uri, "?")
				contenttype := ""

				switch method {
				case "POST":
					if m.Options("file") {
						file, e := os.Open(m.Meta["file"][1])
						m.Assert(e)
						defer file.Close()

						buf := &bytes.Buffer{}
						writer := multipart.NewWriter(buf)

						part, e := writer.CreateFormFile(m.Option("file"), filepath.Base(m.Meta["file"][1]))
						m.Assert(e)

						io.Copy(part, file)
						for i := 0; i < len(arg)-1; i += 2 {
							value := arg[i+1]
							if len(arg[i+1]) > 1 {
								switch arg[i+1][0] {
								case '$':
									value = m.Cap(arg[i+1][1:])
								case '@':
									value = m.Conf(arg[i+1][1:])
								}
							}
							writer.WriteField(arg[i], value)
						}

						contenttype = writer.FormDataContentType()
						body = buf
						writer.Close()
					} else if index > 0 {
						contenttype = "application/x-www-form-urlencoded"
						body = strings.NewReader(uri[index+1:])
						uri = uri[:index]
					}
				}

				req, e := http.NewRequest(method, uri, body)
				m.Assert(e)

				if len(contenttype) > 0 {
					req.Header.Set("Content-Type", contenttype)
				}

				for _, v := range web.cookie {
					req.AddCookie(v)
				}

				res, e := web.client.Do(req)
				m.Assert(e)

				if web.cookie == nil {
					web.cookie = make(map[string]*http.Cookie)
				}
				for _, v := range res.Cookies() {
					web.cookie[v.Name] = v
				}

				for k, v := range res.Header {
					m.Log("info", nil, "%s: %v", k, v)
				}

				if m.Confs("output") {
					if _, e := os.Stat(m.Conf("output")); e == nil {
						name := path.Join(m.Conf("output"), fmt.Sprintf("%d", time.Now().Unix()))
						f, e := os.Create(name)
						m.Assert(e)
						io.Copy(f, res.Body)
						if m.Confs("editor") {
							cmd := exec.Command(m.Conf("editor"), name)
							cmd.Stdin = os.Stdin
							cmd.Stdout = os.Stdout
							cmd.Stderr = os.Stderr
							cmd.Run()
						} else {
							m.Echo("write to %s\n", name)
						}
						return
					}
				}

				buf, e := ioutil.ReadAll(res.Body)
				m.Assert(e)

				if res.Header.Get("Content-Type") == "application/json" {
					result := map[string]interface{}{}
					json.Unmarshal(buf, &result)
					for k, v := range result {
						switch value := v.(type) {
						case string:
							m.Append(k, value)
						case float64:
							m.Append(k, fmt.Sprintf("%d", int(value)))
						default:
							m.Put("append", k, value)
						}
					}
				}

				result := string(buf)
				m.Echo(result)
				m.Append("response", result)
				// }}}
			}},
		"list": &ctx.Command{Name: "list [set|add|del [url]]", Help: "查看、访问、添加url", Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) {
			web, ok := m.Target().Server.(*WEB) // {{{
			m.Assert(ok)

			switch len(arg) {
			case 0:
				for _, k := range web.list_key {
					if v, ok := web.list[k]; ok {
						m.Echo("%s: %s\n", k, v)
					}
				}
			case 1:
				msg := m.Spawn(m.Target()).Cmd("get", web.list[arg[0]])
				m.Copy(msg, "result")
			default:
				switch arg[0] {
				case "add":
					web.list[m.Cap("count")] = arg[1:]
					web.list_key = append(web.list_key, m.Cap("count"))
					m.Capi("count", 1)
				case "del":
					delete(web.list, arg[1])
				case "set":
					web.list[arg[1]] = arg[2:]
				default:
					list := []string{}
					j := 1
					for _, v := range web.list[arg[0]] {
						if v == "_" && j < len(arg) {
							list = append(list, arg[j])
							j++
						} else {
							list = append(list, v)
						}
					}
					for ; j < len(arg); j++ {
						list = append(list, arg[j])
					}

					msg := m.Spawn(m.Target()).Cmd("get", list)
					m.Copy(msg, "result")
				}
			} // }}}
		}},
		"upload": &ctx.Command{Name: "upload file", Help: "上传文件", Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) {
			msg := m.Spawn(m.Target()) // {{{
			msg.Cmd("get", "/upload", "method", "POST", "file", "file", arg[0])
			m.Copy(msg, "result")
			// }}}
		}},
		"/travel": &ctx.Command{Name: "/travel", Help: "文件上传", Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) {
			if !m.Options("module") { //{{{
				m.Option("module", "ctx")
			}

			r := m.Data["request"].(*http.Request)
			w := m.Data["response"].(http.ResponseWriter)

			// 权限检查
			if m.Option("method") == "POST" {
				if m.Options("domain") {
					msg := m.Find("ssh", true)
					msg.Detail(0, "send", "domain", m.Option("domain"), "context", "find", m.Option("module"), m.Option("ccc"))
					if m.Options("name") {
						msg.Add("detail", m.Option("name"))
					}
					if m.Options("value") {
						msg.Add("detail", m.Option("value"))
					}

					msg.CallBack(true, func(sub *ctx.Message) *ctx.Message {
						m.Copy(sub, "result").Copy(sub, "append")
						return nil
					})
					return
				}

				msg := m.Find(m.Option("module"), true)
				if msg == nil {
					return
				}

				switch m.Option("ccc") {
				case "cache":
					m.Echo(msg.Cap(m.Option("name")))
				case "config":
					if m.Options("value") {
						m.Echo(msg.Conf(m.Option("name"), m.Option("value")))
					} else {
						m.Echo(msg.Conf(m.Option("name")))
					}
				case "command":
					msg = msg.Spawn(msg.Target())
					msg.Detail(0, m.Option("name"))
					if m.Options("value") {
						msg.Add("detail", m.Option("value"))
					}

					msg.Cmd()
					m.Copy(msg, "result").Copy(msg, "append")
				}
				return
			}

			// 解析模板
			render := m.Spawn(m.Target()).Put("option", "request", r).Put("option", "response", w)
			defer render.Cmd(m.Conf("travel_main"), m.Conf("travel_tmpl"))

			if msg := m.Find(m.Option("module"), true); msg != nil {
				m.Option("tmpl", "")
				for _, v := range []string{"cache", "config", "command", "module", "domain"} {
					if m.Options("domain") {
						msg = m.Find("ssh", true)
						msg.Detail(0, "send", "domain", m.Option("domain"), "context", "find", m.Option("module"), v)
						msg.CallBack(true, func(sub *ctx.Message) *ctx.Message {
							msg.Copy(sub, "result").Copy(sub, "append")
							return nil
						})
					} else {
						msg = msg.Spawn(msg.Target())
						msg.Cmd("context", "find", m.Option("module"), v)
					}

					if len(msg.Meta["append"]) > 0 {
						render.Option("current_module", m.Option("module"))
						render.Option("current_domain", m.Option("domain"))
						render.Sesss(v, msg).Add("option", "tmpl", v)
					}
				}
			}
			// }}}
		}},
		"/upload": &ctx.Command{Name: "/upload", Help: "文件上传", Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) {
			r := m.Optionv("request").(*http.Request) // {{{
			w := m.Optionv("response").(http.ResponseWriter)

			if !m.Options("dir") {
				m.Option("dir", m.Cap("directory"))
			}

			check := m.Spawn().Cmd("/share")
			if !check.Results(0) {
				m.Copy(check, "append")
				return
			}
			aaa := check.Appendv("aaa").(*ctx.Message)

			// 输出文件
			s, e := os.Stat(m.Option("dir"))
			if m.Assert(e); !s.IsDir() {
				http.ServeFile(w, r, m.Option("dir"))
				return
			}

			// 共享列表
			share := m.Sesss("share", m.Target())
			index := share.Target().Index
			if index != nil && index[aaa.Append("userrole")] != nil {
				for k, v := range index[aaa.Append("userrole")].Index {
					for _, j := range v.Commands {
						for _, n := range j.Shares {
							for _, nn := range n {
								if match, e := regexp.MatchString(nn, m.Option("dir")); m.Assert(e) && match {
									share.Add("append", "group", k)
									share.Add("append", "value", nn)
									share.Add("append", "delete", "delete")
								}
							}
						}
					}
				}
			}
			share.Sort("value", "string")
			share.Sort("argument", "string")

			// 输出目录
			fs, e := ioutil.ReadDir(m.Option("dir"))
			m.Assert(e)
			fs = append(fs, s)
			list := m.Sesss("list", m.Target())
			list.Option("dir", m.Option("dir"))

			for _, v := range fs {
				name := v.Name()
				if v == s {
					if name == m.Option("dir") {
						continue
					}
					name = ".."
				} else if name[0] == '.' {
					continue
				}

				if v.IsDir() {
					name += "/"
				}

				list.Add("append", "time_i", fmt.Sprintf("%d", v.ModTime().Unix()))
				list.Add("append", "size_i", fmt.Sprintf("%d", v.Size()))
				list.Add("append", "time", v.ModTime().Format("2006-01-02 15:04:05"))
				list.Add("append", "size", kit.FmtSize(v.Size()))
				list.Add("append", "name", name)
				list.Add("append", "path", path.Join(m.Option("dir"), name))
			}

			// 目录排序
			max := true
			if i, e := strconv.Atoi(m.Option("order")); e == nil {
				max = i%2 == 1
			}
			switch m.Option("list") {
			case "name":
				if max {
					list.Sort(m.Option("list"), "string")
				} else {
					list.Sort(m.Option("list"), "string_r")
				}
			case "size", "time":
				if max {
					list.Sort(m.Option("list")+"_i", "int")
				} else {
					list.Sort(m.Option("list")+"_i", "int_r")
				}
			}
			list.Meta["append"] = list.Meta["append"][2:]
			delete(list.Meta, "time_i")
			delete(list.Meta, "size_i")

			// 执行命令
			switch m.Option("cmd") {
			case "git":
				git := m.Sesss("git", m.Target())

				branch := m.Find("nfs").Cmd("git", "-C", m.Option("dir"), "branch")
				git.Option("branch", branch.Result(0))

				status := m.Find("nfs").Cmd("git", "-C", m.Option("dir"), "status")
				git.Option("status", status.Result(0))
			}

			m.Append("title", "upload")
			m.Append("tmpl", "userinfo", "share", "list", "git", "upload", "create")
			m.Append("template", m.Conf("upload_main"), m.Conf("upload_tmpl"))
			// }}}
		}},
		"/create": &ctx.Command{Name: "/create", Help: "创建目录或文件", Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) {
			if check := m.Spawn().Cmd("/share"); !check.Results(0) { // {{{
				m.Copy(check, "append")
				return
			}

			r := m.Optionv("request").(*http.Request)
			if m.Option("method") == "POST" {
				if m.Options("filename") { //添加文件或目录
					name := path.Join(m.Option("dir"), m.Option("filename"))
					if _, e := os.Stat(name); e != nil {
						if m.Options("content") {
							f, e := os.Create(name)
							m.Assert(e)
							defer f.Close()

							_, e = f.WriteString(m.Option("content"))
							m.Assert(e)
						} else {
							e = os.Mkdir(name, 0766)
							m.Assert(e)
						}
						m.Append("message", name, " create success!")
					} else {
						m.Append("message", name, " already exist!")
					}
				} else { //上传文件
					file, header, e := r.FormFile("file")
					m.Assert(e)

					name := path.Join(m.Option("dir"), header.Filename)

					if _, e := os.Stat(name); e != nil {
						f, e := os.Create(name)
						m.Assert(e)
						defer f.Close()

						_, e = io.Copy(f, file)
						m.Assert(e)
						m.Append("message", name, " upload success!")
					} else {
						m.Append("message", name, " already exist!")
					}
				}
			}
			m.Append("redirect", m.Option("referer"))
			// }}}
		}},
		"/share": &ctx.Command{Name: "/share", Help: "资源共享", Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) {
			check := m.Spawn().Cmd("/check", "command", "/share", "dir", m.Option("dir")) // {{{
			if !check.Results(0) {
				m.Copy(check, "append")
				return
			}

			msg := check.Appendv("aaa").(*ctx.Message).Spawn(m.Target())
			if m.Options("shareto") {
				msg.Cmd("right", "add", m.Option("shareto"), "command", "/share", "dir", m.Option("dir"))
			}
			if m.Options("notshareto") {
				msg.Cmd("right", "del", m.Option("notshareto"), "command", "/share", "dir", m.Option("dir"))
			}
			m.Echo("ok")
			// }}}
		}},
		"/check": &ctx.Command{Name: "/check cache|config|command name args", Help: "权限检查, cache|config|command: 接口类型, name: 接口名称, args: 其它参数", Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) {
			w := m.Optionv("response").(http.ResponseWriter) //{{{
			if login := m.Spawn().Cmd("/login"); login.Has("redirect") {
				if msg := m.Spawn().Cmd("right", "check", login.Append("userrole"), arg); msg.Results(0) {
					m.Copy(login, "append").Echo(msg.Result(0))
					return
				}
				w.WriteHeader(http.StatusForbidden)
				m.Append("message", "please contact manager")
				m.Echo("no")
				return
			} else {
				m.Copy(login, "append").Echo("no")
			}
			// }}}
		}},
		"/login": &ctx.Command{Name: "/login", Help: "用户登录", Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) {
			w := m.Optionv("response").(http.ResponseWriter) // {{{

			if m.Options("sessid") {
				if aaa := m.Find("aaa").Cmd("login", m.Option("sessid")); aaa.Results(0) {
					m.Append("redirect", m.Option("referer"))
					return
				}
			}

			if m.Options("username") && m.Options("password") {
				if aaa := m.Find("aaa").Cmd("login", m.Option("username"), m.Option("password")); aaa.Results(0) {
					http.SetCookie(w, &http.Cookie{Name: "sessid", Value: aaa.Result(0)})
					m.Append("redirect", m.Option("referer"))
					return
				}
			}

			w.WriteHeader(http.StatusUnauthorized)
			m.Append("template", "login.html")
			// }}}
		}},
		"/render": &ctx.Command{Name: "/render [main [tmpl]]", Help: "模板响应, main: 模板入口, tmpl: 附加模板", Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) {
			w := m.Optionv("response").(http.ResponseWriter) // {{{

			tpl := template.Must(template.New("render").Funcs(ctx.CGI).ParseGlob(path.Join(m.Conf("template_dir"), m.Conf("common_tmpl"))))
			if len(arg) > 1 {
				tpl = template.Must(tpl.ParseGlob(path.Join(m.Conf("template_dir"), arg[1])))
			}

			main := m.Conf("common_main")
			if len(arg) > 0 {
				main = arg[0]
			}

			w.Header().Add("Content-Type", "text/html")
			m.Assert(tpl.ExecuteTemplate(w, main, m.Message()))
			// }}}
		}},
		"/json": &ctx.Command{Name: "/json", Help: "json响应", Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) {
			w := m.Optionv("response").(http.ResponseWriter) // {{{

			meta := map[string][]string{}
			if len(m.Meta["append"]) > 0 {
				meta["append"] = m.Meta["append"]
				for _, v := range m.Meta["append"] {
					meta[v] = m.Meta[v]
				}
			}

			if b, e := json.Marshal(meta); m.Assert(e) {
				w.Header().Set("Content-Type", "application/javascript")
				w.Write(b)
			}
			// }}}
		}},
		"temp": &ctx.Command{Name: "temp", Help: "应用示例", Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) {
			msg := m.Spawn(m.Target())
			question := []string{}
			for i := 1; i < 21; i++ {
				question = append(question, fmt.Sprintf("{\"type\":\"1001\",\"title\":{\"text\":\"第%d题\"}}", i))
			}
			qs := "[" + strings.Join(question, ",") + "]"

			msg.Cmd("get", "method", "POST", "evaluating_add/", "questions", qs)
			m.Add("append", "hi", "hello")
		}},
		"/demo": &ctx.Command{Name: "/demo", Help: "应用示例", Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) {
			w := m.Data["response"].(http.ResponseWriter)

			w.Header().Add("Content-Type", "text/html")
			tmpl := template.Must(template.New("fuck").Funcs(ctx.CGI).ParseGlob(m.Conf("template_dir") + "/common/*.html"))
			m.Assert(tmpl)

			m.Option("message", "hello")
			tmpl.ExecuteTemplate(w, "head", m)
			tmpl.ExecuteTemplate(w, "tail", m.Meta)
		}},
	},
}

func init() {
	web := &WEB{}
	web.Context = Index
	ctx.Index.Register(Index, web)
}
