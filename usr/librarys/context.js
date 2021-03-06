ctx = context = (function(kit) {var ctx = {__proto__: kit,
    Run: shy("请求后端", {order: 0}, function(event, dataset, cmd, cb) {
        var msg = ctx.Event(event)

        var option = {"cmds": cmd}
        msg.option && msg.option.forEach(function(item) {
            msg[item] && (option[item] = msg[item])
        })
        for (var k in dataset) {
            option[k] = dataset[k].split(",")
        }

        var what = ++arguments.callee.meta.order

        msg.option = []
        for (var k in option) {
            msg.option.push(k)
            msg[k] = option[k]
        }
        msg.detail = ["run", what].concat(option.group).concat(option.names).concat(option.cmds)
        kit.Log(msg.detail.concat([msg]))

        kit.History("run", -1, option)
        this.POST("", option, function(msg) {
            kit.Log("run", what, "result", msg.result? msg.result[0]: "", msg)
            typeof cb == "function" && cb(msg)
        }, msg), delete(event.msg)
    }),
    Event: shy("封装事件", {order: 0}, function(event, msg, proto) {
        if (event.msg && !msg) {return event.msg}

        event.msg = msg = msg || {}, proto = proto || {}, msg.__proto__ = proto, proto.__proto__ = {
            Copy: function(res) {
                res.result && (msg.result = res.result)
                res.append && (msg.append = res.append) && res.append.forEach(function(item) {
                    res[item] && (msg[item] = res[item])
                })
                return msg
            },
            Push: function(key, value) {msg.append = msg.append || []
                msg[key]? msg[key].push(value): (msg[key] = [value], msg.append.push(key))
                return msg
            },
            Echo: function(res) {
                kit.notNone(res) && (msg.result = (msg.result || []).concat(kit._call(kit.List, arguments)))
                return msg
            },
            Format: function() {
                return msg.append && msg.append[0]? ["table", JSON.stringify(msg.Table())]: ["code", msg.result? msg.result.join(""): ""]
            },
            Option: function(key, val) {
                if (val == undefined) {return msg[key]}
                msg.option = msg.option || []
                kit.List(msg.option, function(k) {
                    if (k == key) {return k}
                }).length > 0 || msg.option.push(key)
                msg[key] = kit.List(arguments).slice(1)
            },
            Result: function() {return msg.result? msg.result.join(""): ""},
            Results: function() {return kit.Color(msg.Result().replace(/</g, "&lt;").replace(/>/g, "&gt;"))},
            Table: function(cb) {if (!msg.append || !msg.append.length || !msg[msg.append[0]]) {return}
                var max = "", len = 0
                kit.List(msg.append, function(key, index) {
                    if (msg[key].length > len) {
                        max = key, len = msg[key].length
                    }
                })

                return kit.List(msg[max], function(value, index, array) {var one = {}
                    msg.append.forEach(function(key) {one[key] = msg[key][index]||""})
                    return kit._call(cb, [one, index, array])
                })
            },
        }, msg.event = event

        kit.Log("event", ++arguments.callee.meta.order, event.type, (proto.name||[document.title]).join("."), msg)
        return msg
    }),
    Share: shy("共享链接", function(objs, clear) {var obj = objs || {}
        !clear && kit.Item(this.Search(), function(key, value) {obj[key] || (obj[key] = value)})
        return location.origin+location.pathname+(objs? "?"+kit.Item(obj, function(key, value) {
            return kit.List(value, function(value) {return key+"="+encodeURIComponent(value)}).join("&")
        }).join("&"): "")
    }),

    Search: shy("请求变量", function(key, value) {var args = {}
        location.search && location.search.slice(1).split("&").forEach(function(item) {var x = item.split("=")
            x[1] != "" && (args[x[0]] = decodeURIComponent(x[1]))
        })

        if (typeof key == "object") {
            kit.Item(key, function(key, value) {
                if (kit.notNone(value)) {args[key] = value}
            })
        } else if (kit.isNone(key)) {
            return args
        } else if (kit.isNone(value)) {
            return args[key] || ctx.Cookie(key)
        } else {
            args[key] = value
        }

        return location.search = kit.Item(args, function(key, value) {
            return key+"="+encodeURIComponent(value)
        }).join("&")
    }),
    Cookie: shy("会话变量", function(key, value, path) {
        function set(k, v) {document.cookie = k+"="+v+";path="+(path||"/")}

        if (typeof key == "object") {
            for (var k in key) {set(k, key[k])}
            key = null
        }
        if (kit.isNone(key)) {var cs = {}
            document.cookie.split("; ").forEach(function(item) {
                var cookie = item.split("=")
                cs[cookie[0]] = cookie[1]
            })
            return cs
        }

        kit.notNone(value) && set(key, value)
        var result = (new RegExp(key+"=([^;]*);?")).exec(document.cookie)
        return result && result.length > 0? result[1]: ""
    }),
    Upload: shy("上传文件", function(form, file, cb, detail) {
        var data = new FormData()
        for (var k in form) {data.append(k, form[k])}
        data.append("upload", file)

        var xhr = new XMLHttpRequest()
        xhr.onreadystatechange = function() {
            if (xhr.readyState != 4) {return}
            if (xhr.status != 200) {return}
        }
        xhr.upload.onprogress = function(event) {kit._call(detail, [event])}
        xhr.onload = function(event) {kit._call(cb, [event, ctx.Event(event, JSON.parse(xhr.responseText||'{"result":[]}'), {name: [document.title]})])}
        xhr.open("POST", "/upload", true)
        xhr.send(data)
    }),
    POST: shy("请求后端", {order: 0}, function(url, form, cb, msg) {
        var args = kit.Items(form, function(value, index, key) {
            return key+"="+encodeURIComponent(value)
        })

        var xhr = new XMLHttpRequest()
        xhr.onreadystatechange = function() {
            if (xhr.readyState != 4) {return}
            if (xhr.status != 200) {return}

            try {
                var res = JSON.parse(xhr.responseText||'[{"result":[]}]')
                res.length > 0 && res[0] && (res = res[0])

                if (res.download_file) {
                    window.open(res.download_file.join(""))
                } else if (res.page_redirect) {
                    location.href = res.page_redirect.join("")
                } else if (res.page_refresh) {
                    location.reload()
                }
            } catch (e) {
                var res = {"result": [xhr.responseText]}
            }

            typeof cb == "function" && cb(msg.Copy(res))
        }

        xhr.open("POST", url)
        xhr.setRequestHeader("Content-Type", "application/x-www-form-urlencoded")
        xhr.setRequestHeader("Accept", "application/json")
        xhr.send(args.join("&"))
        ++arguments.callee.meta.order
    }),
    WSS: shy("响应后端", {order: 0, wssid: ""}, function(cb, onerror, onclose, onopen) {var meta = arguments.callee.meta
        var s = new WebSocket(location.protocol.replace("http", "ws")+"//"+location.host+"/wss?wssid="+meta.wssid)
        s.onerror = onerror, s.onclose = onclose, s.onopen = onopen
        s.onmessage = function(event) {var order = ++meta.order
            try {
                var msg = JSON.parse(event.data||'{}')
            } catch (e) {
                var msg = {"result": [event.data]}
            }

            // Event入口 -1.0
            msg = ctx.Event(event, msg, {name: [document.title, "wss", msg.detail[0]], Order: order, Reply: function(msg) {
                kit.Log(["wss", order, "result"].concat(msg.result).concat([msg]))
                delete(msg.event), s.send(JSON.stringify(msg))
            }})

            try {
                kit.Log(["wss", order].concat(msg.detail).concat([msg]))
                kit._call(cb, [event, msg])
            } catch (e) {
                msg.Reply(kit.Log("err", e))
            }
        }
        return s
    }),
}; return ctx})(kit)
