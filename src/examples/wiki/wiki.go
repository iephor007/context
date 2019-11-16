package wiki

import (
	"github.com/gomarkdown/markdown"

	"contexts/ctx"
	"contexts/web"
	"toolkit"

	"bytes"
	"encoding/json"
	"path"
	"strings"
	"text/template"
)

var Index = &ctx.Context{Name: "wiki", Help: "文档中心",
	Caches: map[string]*ctx.Cache{},
	Configs: map[string]*ctx.Config{
		"login": {Name: "login", Value: map[string]interface{}{"check": "false"}, Help: "用户登录"},
		"componet": {Name: "componet", Value: map[string]interface{}{
			"index": []interface{}{
				map[string]interface{}{"name": "wiki",
					"tmpl": "head", "metas": []interface{}{map[string]interface{}{
						"name": "viewport", "content": "width=device-width, initial-scale=0.7, user-scalable=no",
					}}, "favicon": "favicon.ico", "styles": []interface{}{
						"example.css", "wiki.css",
					}},
				map[string]interface{}{"name": "header",
					"tmpl": "fieldset", "view": "Header", "init": "initHeader",
				},
				map[string]interface{}{"name": "tree",
					"tmpl": "fieldset", "view": "Tree", "init": "initTree",
					"ctx": "web.wiki", "cmd": "tree",
				},
				map[string]interface{}{"name": "text",
					"tmpl": "fieldset", "view": "Text", "init": "initText",
					"ctx": "web.wiki", "cmd": "text",
				},
				map[string]interface{}{"name": "footer",
					"tmpl": "fieldset", "view": "Footer", "init": "initFooter",
				},
				map[string]interface{}{"name": "tail",
					"tmpl": "tail", "scripts": []interface{}{
						"toolkit.js", "context.js", "example.js", "wiki.js",
					},
				},
			},
		}, Help: "组件列表"},

		"level": {Name: "level", Value: "usr/local/wiki", Help: "文档路径"},
		"class": {Name: "class", Value: "", Help: "文档目录"},
		"favor": {Name: "favor", Value: "index.md", Help: "默认文档"},

		"story": {Name: "story", Value: map[string]interface{}{
			"data": map[string]interface{}{},
			"node": map[string]interface{}{},
			"head": map[string]interface{}{},
		}, Help: "故事会"},
	},
	Commands: map[string]*ctx.Command{
		"tree": {Name: "tree", Help: "目录", Form: map[string]int{"level": 1, "class": 1}, Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) (e error) {
			m.Cmdy("nfs.dir", path.Join(m.Confx("level"), m.Confx("class", arg, 0)),
				"time", "size", "line", "file", "dir_sort", "time", "time_r")
			return
		}},
		"text": {Name: "text", Help: "文章", Form: map[string]int{"level": 1, "class": 1, "favor": 1}, Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) (e error) {
			which := m.Cmdx("nfs.path", path.Join(m.Confx("level"), m.Confx("class", arg, 1), m.Confx("favor", arg, 0)))

			buffer := bytes.NewBuffer([]byte{})
			tmpl := template.New("render").Funcs(*ctx.LocalCGI(m, c))

			tmpl = template.Must(tmpl.ParseGlob(path.Join(m.Conf("route", "template_dir"), "/*.tmpl")))
			tmpl = template.Must(tmpl.ParseGlob(path.Join(m.Conf("route", "template_dir"), m.Cap("route"), "/*.tmpl")))
			tmpl = template.Must(tmpl.ParseFiles(which))

			m.Optionv("tmpl", tmpl)
			m.Assert(tmpl.ExecuteTemplate(buffer, m.Option("filename", path.Base(which)), m))
			m.Echo(string(markdown.ToHTML(buffer.Bytes(), nil, nil)))
			return
		}},
		"note": {Name: "note file", Help: "便签", Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) (e error) {
			if len(arg) == 0 {
				m.Cmd("tree")
				return
			}

			switch arg[0] {
			case "favor", "commit":
				m.Cmd("story", arg[0], arg[1:])
			default:
				m.Cmd(kit.Select("tree", "text", strings.HasSuffix(arg[0], ".md")), arg[0])
			}
			return
		}},
		"story": {Name: "story commit story scene enjoy happy", Help: "故事会", Hand: func(m *ctx.Message, c *ctx.Context, cmd string, arg ...string) (e error) {
			switch arg[0] {
			case "favor":
				if len(arg) < 4 {
					m.Cmdy("ssh.data", "show", arg[1:])
					break
				}

				head := kit.Hashs(arg[2], arg[4])
				prev := m.Conf(cmd, []string{"head", head, "node"})
				m.Cmdy("ssh.data", "insert", arg[1], "story", arg[2], "scene", arg[3], "enjoy", arg[4], "node", prev)

			case "commit":
				head := kit.Hashs(arg[1], arg[3])
				prev := m.Conf(cmd, []string{"head", head, "node"})
				m.Log("info", "head: %v %#v", head, prev)

				if len(arg) > 4 {
					data := kit.Hashs(arg[4])
					m.Log("info", "data: %v %v", data, arg[4])
					if m.Conf(cmd, []string{"node", prev, "data"}) != data {
						m.Conf(cmd, []string{"data", data}, arg[4])

						meta := map[string]interface{}{
							"time":  m.Time(),
							"story": arg[1],
							"scene": arg[2],
							"enjoy": arg[3],
							"data":  data,
							"prev":  prev,
						}
						node := kit.Hashs(kit.Format(meta))
						m.Log("info", "node: %v %v", node, meta)
						m.Conf(cmd, []string{"node", node}, meta)

						m.Log("info", "head: %v %v", head, node)
						m.Conf(cmd, []string{"head", head, "node"}, node)
						m.Echo("%v", kit.Formats(meta))
						break
					}
				}

				for prev != "" {
					node := m.Confm(cmd, []string{"node", prev})
					m.Push("node", kit.Short(prev, 6))
					m.Push("time", node["time"])
					m.Push("data", m.Conf(cmd, []string{"data", kit.Format(node["data"])}))
					prev = kit.Format(node["prev"])
				}
				m.Table()
				return

			case "branch":
				m.Confm(cmd, "head", func(key string, value map[string]interface{}) {
					node := kit.Format(value["node"])
					m.Push("key", kit.Short(key, 6))
					m.Push("story", m.Conf(cmd, []string{"node", node, "story"}))
					m.Push("scene", m.Conf(cmd, []string{"node", node, "scene"}))
					m.Push("enjoy", m.Conf(cmd, []string{"node", node, "enjoy"}))
					m.Push("node", kit.Short(value["node"], 6))
				})
				m.Table()
			case "remote":
			}
			return
		}},
		"table": {Name: "table", Help: "表格", Hand: func(m *ctx.Message, c *ctx.Context, cmd string, arg ...string) (e error) {
			switch len(arg) {
			case 0:
				return
			case 2:
				if arg[1] != "head" {
					break
				}
				fallthrough
			case 1:
				arg = []string{arg[0], "head", kit.Hashs(m.Option("filename"), arg[1])}
				fallthrough
			default:
				switch arg[1] {
				case "name":
					arg = []string{arg[0], "head", kit.Hashs(m.Option("filename"), arg[2])}
					fallthrough
				case "head":
					arg = []string{arg[0], "node", m.Conf("story", []string{"head", arg[2], "node"})}
					fallthrough
				case "node":
					arg = []string{arg[0], "data", m.Conf("story", []string{"node", arg[2], "data"})}
					fallthrough
				case "data":
					arg = []string{arg[0], m.Conf("story", []string{"data", arg[2]})}
				}
			}

			m.Option("scene", cmd)
			m.Option("enjoy", arg[0])
			m.Option("happy", arg[1])

			head := []string{}
			for i, l := range strings.Split(strings.TrimSpace(arg[1]), "\n") {
				if i == 0 {
					head = kit.Split(l, ' ', 100)
					continue
				}
				for j, v := range strings.Split(l, " ") {
					m.Push(head[j], v)
				}
			}
			return
		}},
		"runs": {Name: "run", Help: "便签", Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) (e error) {
			m.Cmdy(arg).Set("append")
			return
		}},
		"run": {Name: "run", Help: "便签", Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) (e error) {
			m.Cmdy(arg)
			return
		}},
		"time": {Name: "time", Help: "便签", Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) (e error) {
			m.Cmdy("cli.time", "show").Set("append")
			return
		}},

		"svg": {Name: "svg", Help: "绘图", Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) (e error) {
			m.Echo(arg[0])
			return
		}},

		"xls": {Name: "xls", Help: "表格", Hand: func(m *ctx.Message, c *ctx.Context, key string, arg ...string) (e error) {
			switch len(arg) {
			case 0:
				m.Cmdy("ssh.data", "show", "xls")
				m.Meta["append"] = []string{"id", "title"}

			case 1:
				var data map[int]map[int]string
				what := m.Cmd("ssh.data", "show", "xls", arg[0], "format", "object").Append("content")
				json.Unmarshal([]byte(what), &data)

				max, n := 0, 0
				for i, v := range data {
					if i > n {
						n = i
					}
					for i := range v {
						if i > max {
							max = i
						}
					}
				}
				m.Log("info", "m: %d n: %d", max, n)

				for k := 0; k < n+2; k++ {
					for i := 0; i < max+2; i++ {
						m.Push(kit.Format(k), kit.Format(data[k][i]))
					}
				}

			case 2:
				m.Cmdy("ssh.data", "insert", "xls", "title", arg[0], "content", arg[1])

			default:
				data := map[int]map[int]string{}
				what := m.Cmd("ssh.data", "show", "xls", arg[0], "format", "object").Append("content")
				json.Unmarshal([]byte(what), &data)

				for i := 1; i < len(arg)-2; i += 3 {
					if _, ok := data[kit.Int(arg[i])]; !ok {
						data[kit.Int(arg[i])] = make(map[int]string)
					}
					data[kit.Int(arg[i])][kit.Int(arg[i+1])] = arg[i+2]
				}
				m.Cmdy("ssh.data", "update", "xls", arg[0], "content", kit.Format(data))
			}
			return
		}},
	},
}

func init() {
	web.Index.Register(Index, &web.WEB{Context: Index})
}
