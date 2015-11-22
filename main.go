package main

import (
	"crypto/md5"
	"flag"
	"fmt"
	"github.com/pivotal-golang/bytefmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/fcgi"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var err error

var (
	httpAddr = flag.String("address", "127.0.0.1", "Http address")
	httpPort = flag.Int("port", 8080, "Http port")
	webPath  = flag.String("webpath", "/storage/", "Web Path")
	osPath   = flag.String("ospath", "./storage", "OS Path")
	login    = flag.String("login", "admin", "Web login")
	password = flag.String("password", "admin123", "Web password")
	useFcgi  = flag.Bool("fcgi", false, "FastCGI")
)

func upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fmt.Fprint(w, `<html><head><title>Upload</title></head>
<body>
	<p>Upload an file to storage:</p>
	<form  method="POST" enctype="multipart/form-data">
		<input type="file" name="image">
		<input type="submit" value="Upload">
	</form>
</body></html>`)
		return
	}

	infile, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer infile.Close()

	bs, err := ioutil.ReadAll(infile)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	hash := md5.New()
	io.WriteString(hash, string(bs))
	hashed := hash.Sum(nil)

	info := strings.Split(strings.ToLower(header.Filename), ".")

	var ext string
	if len(info) > 1 {
		if len(info) > 2 && info[len(info)-2] == "tar" && info[len(info)-1] == "gz" {
			ext = ".tar.gz"
		} else {
			ext = fmt.Sprintf(".%s", info[len(info)-1])
		}
	}

	filepath := fmt.Sprintf("%s/%x/%x/%x%s", *osPath, hashed[0:1], hashed[1:2], hashed[2:], ext)

	err = os.MkdirAll(fmt.Sprintf("%s/%x/%x/", *osPath, hashed[0:1], hashed[1:2]), 0755)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = ioutil.WriteFile(filepath, bs, 0644)

	path := fmt.Sprintf("%s/%x/%x/%x%s", *webPath, hashed[0:1], hashed[1:2], hashed[2:], ext)
	http.Redirect(w, r, path, 302)
}

func index(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Hello!")
}

func list(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	type Web struct {
		List []struct {
			Name string
			Size string
			Time string
		}
		Version  string
		LoadTime string
	}

	var fileList Web

	fileList.Version = strings.Title(runtime.Version())

	err = filepath.Walk(*OsPath, func(path string, f os.FileInfo, err error) error {
		finfo, err := os.Stat(path)
		if err != nil {
			return err
		}
		if !finfo.IsDir() {
			path = strings.TrimPrefix(path, *osPath)
			if strings.HasPrefix(path, "/") {
				path = strings.TrimPrefix(path, "/")
			}
			timearr := strings.Split(fmt.Sprintf("%q", finfo.ModTime()), " ")
			thisfile := struct {
				Name string
				Size string
				Time string
			}{
				fmt.Sprintf("%s%s", *webPath, path),
				bytefmt.ByteSize(uint64(finfo.Size())),
				fmt.Sprintf("%s %s", timearr[0][1:], timearr[1]),
			}
			fileList.List = append(fileList.List, thisfile)
		}
		return nil
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	const tpl = `<html><head><title>List</title></head>
<body>
<table>{{range .List}}
<tr><td><a href="{{.Name}}">{{.Name}}</a></td><td>{{.Time}}</td><td>{{.Size}}</td></tr>
{{end}}</table>
<hr>
<small>Go: {{.Version}} | GT: {{.LoadTime}}</small>
</body></html>`
	t, err := template.New("webpage").Parse(tpl)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fileList.LoadTime = fmt.Sprintf("%q", time.Since(startTime))
	err = t.Execute(w, fileList)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func main() {
	flag.Parse()
	fs := HideDir(http.FileServer(http.Dir(*osPath)))
	http.Handle(*webPath, http.StripPrefix(*webPath, fs))
	http.HandleFunc("/", index)
	http.HandleFunc("/list", BasicAuth(list, *login, *password))
	http.HandleFunc("/upload", Logger(BasicAuth(upload, *login, *password)))
	bind := fmt.Sprintf("%s:%d", *httpAddr, *httpPort)
	log.Println("Starting on", bind)
	if *useFcgi {
		l, err := net.Listen("tcp", bind)
		if err != nil {
			panic(err.Error())
			return
		}
		fcgi.Serve(l, nil)
	} else {
		http.ListenAndServe(bind, nil)
	}
}
