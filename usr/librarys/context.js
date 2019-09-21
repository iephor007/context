ctx = context = {
    Run: function(dataset, cmd, cb) {
        var option = {"cmds": cmd}
        for (var k in dataset) {
            option[k] = dataset[k].split(",")
        }

        var event = window.event
        this.POST("", option, function(msg) {
            msg[0] && (msg = msg[0])
            msg.Result = msg.result? msg.result.join(""): ""
            msg.Results = function() {
                var s = msg.Result
                s = s.replace(/</g, "&lt;")
                s = s.replace(/>/g, "&gt;")
                s = kit.Color(s)
                return s
            }
            msg.event = event
            typeof cb == "function" && cb(msg || {})
        })
    },
    Runs: function(form, cb) {
        var data = {}
        for (var key in form.dataset) {
            data[key] = form.dataset[key]
        }
        for (var i = 0; i < form.length; i++) {
            if (form[i].name) {
                data[form[i].name] = form[i].value
            }
        }
        this.Run(data, [], cb || form.ondaemon)
    },
    Table: function(msg, cb) {
        var ret = []
        if (!msg || !msg.append || !msg.append.length || !msg[msg.append[0]]) {
            return ret
        }

        var ncol = msg.append.length
        var nrow = msg[msg.append[0]].length
        for (var i = 0; i < nrow; i++) {
            var one = {}
            for (var j = 0; j < ncol; j++) {
                one[msg.append[j]] = msg[msg.append[j]][i]
            }
            ret.push(one)
        }

        var list = []
        typeof cb == "function" && ret.forEach(function(value, index, array) {
            var item = cb(value, index, array)
            item && list.push(item)
        })
        if (list.length > 0) {
            return list
        }
        return ret
    },
    Tables: function(msg, cb) {
        var ret = []
        if (!msg || !msg.append || !msg.append.length || !msg[msg.append[0]]) {
            return ret
        }
        ret.push(msg.append)

        var ncol = msg.append.length
        var nrow = msg[msg.append[0]].length
        for (var i = 0; i < nrow; i++) {
            var one = []
            for (var j = 0; j < ncol; j++) {
                one.push(msg[msg.append[j]][i])
            }
            ret.push(one)
        }

        var list = []
        typeof cb == "function" && ret.forEach(function(value, index, array) {
            var item = cb(value, index, array)
            item && list.push(item)
        })
        if (list.length > 0) {
            return list
        }
        return ret
    },
    Share: function(objs) {
        var args = this.Search()
        for (var k in objs) {
            args[k] = objs[k]
        }

        var as = []
        for (var k in args) {
            if (typeof args[k] == "object") {
                for (var i = 0; i < args[k].length; i++) {
                    as.push(k+"="+encodeURIComponent(args[k][i]));
                }
            } else {
                as.push(k+"="+encodeURIComponent(args[k]));
            }
        }
        var arg = as.join("&");
        return location.origin+location.pathname+"?"+arg
    },

    Search: function(key, value) {
        var args = {}
        var search = location.search.split("?")
        if (search.length > 1) {
            var searchs = search[1].split("&")
            for (var i = 0; i < searchs.length; i++) {
                var keys = searchs[i].split("=")
                if (keys[1] == "") {continue}
                args[keys[0]] = decodeURIComponent(keys[1])
            }
        }

        if (key == undefined) {
            return args
        } else if (typeof key == "object") {
            for (var k in key) {
                if (key[k] != undefined) {
                    args[k] = key[k]
                }
            }
        } else if (value == undefined) {
            return args[key] || this.Cookie(key)
        } else {
            args[key] = value
        }

        var arg = []
        for (var k in args) {
            arg.push(k+"="+encodeURIComponent(args[k]))
        }
        location.search = arg.join("&");
        return value
    },
    Cookie: function(key, value) {
        if (key == undefined) {
            cs = {}
            cookies = document.cookie.split("; ")
            for (var i = 0; i < cookies.length; i++) {
                cookie = cookies[i].split("=")
                cs[cookie[0]] = cookie[1]
            }
            return cs
        }
        if (typeof key == "object") {
            for (var k in key) {
                document.cookie = k+"="+key[k];
            }
            return this.Cookie()
        }
        if (value == undefined) {
            var pattern = new RegExp(key+"=([^;]*);?")
            var result = pattern.exec(document.cookie)
            return result && result.length > 0? result[1]: ""
        }
        document.cookie = key+"="+value+";path=/"
        return this.Cookie(key)
    },
    POST: function(url, form, cb) {
        var args = []
        for (var k in form) {
            if (form[k] instanceof Array) {
                for (i in form[k]) {
                    args.push(k+"="+encodeURIComponent(form[k][i]))
                }
            } else if (form[k] != undefined) {
                args.push(k+"="+encodeURIComponent(form[k]))
            }
        }

        var xhr = new XMLHttpRequest()
        xhr.onreadystatechange = function() {
            if (xhr.readyState != 4) {
                return
            }
            if (xhr.status != 200) {
                return
            }

            try {
                var msg = JSON.parse(xhr.responseText||'{"result":[]}')
            } catch (e) {
                var msg = {"result": [xhr.responseText]}
            }

            if (msg.download_file) {
                window.open(msg.download_file.join(""))
            } else if (msg.page_redirect) {
                location.href = msg.page_redirect.join("")
            } else if (msg.page_refresh) {
                location.reload()
            }
            typeof cb == "function" && cb(msg)
        }

        xhr.open("POST", url)
        xhr.setRequestHeader("Content-Type", "application/x-www-form-urlencoded")
        xhr.setRequestHeader("Accept", "application/json")
        xhr.send(args.join("&"))
    },
    Upload: function(file, cb, detail) {
        var data = new FormData()
        data.append("upload", file)

        var xhr = new XMLHttpRequest()
        xhr.onload = function(event) {
            var msg = JSON.parse(xhr.responseText||'{"result":[]}')
            typeof cb == "function" && cb(event, msg)
        }

        xhr.onreadystatechange = function() {
            if (xhr.readyState != 4) {
                return
            }
            if (xhr.status != 200) {
                return
            }
        }

        xhr.upload.onprogress = function(event) {
            typeof detail == "function" && detail(event)
        }

        xhr.open("POST", "/upload", true)
        xhr.send(data)
    },
}
