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

type webFile struct {
	Name  string
	Size  string
	Time  string
	Today bool
}

type webList struct {
	List       []webFile
	Version    string
	LoadTime   string
	TotalCount uint32
	TotalSize  string
}

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
			ext = fmt.Sprintf(".%s.%s", info[len(info)-2], info[len(info)-1])
		} else {
			ext = fmt.Sprintf(".%s", info[len(info)-1])
		}
	}

	filepath := fmt.Sprintf("%s/%x/%x/%x%s", *osPath, hashed[0:1], hashed[1:2], hashed[2:], ext)

	if err = os.MkdirAll(fmt.Sprintf("%s/%x/%x/", *osPath, hashed[0:1], hashed[1:2]), 0755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err = ioutil.WriteFile(filepath, bs, 0644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	path := fmt.Sprintf("%s/%x/%x/%x%s", *webPath, hashed[0:1], hashed[1:2], hashed[2:], ext)
	http.Redirect(w, r, path, 302)
}

func index(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Hello!")
}

func list(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	var fileList webList
	var totalSize uint64

	fileList.Version = strings.Title(strings.TrimPrefix(runtime.Version(), "go"))
	fileList.TotalCount = 0
	totalSize = 0

	if err = filepath.Walk(*osPath, func(path string, f os.FileInfo, err error) error {
		finfo, err := os.Stat(path)
		if err != nil {
			return err
		}
		if !finfo.IsDir() {
			fileList.TotalCount++
			totalSize += uint64(finfo.Size())
			path = strings.TrimPrefix(path, *osPath)
			smallPath := strings.TrimPrefix(*osPath, "./")
			if strings.HasPrefix(path, smallPath) {
				path = strings.TrimPrefix(path, smallPath)
			}
			if strings.HasPrefix(path, "/") {
				path = strings.TrimPrefix(path, "/")
			}
			timearr := strings.Split(fmt.Sprintf("%q", finfo.ModTime()), " ")
			thisfile := webFile{
				Name: fmt.Sprintf("%s%s", *webPath, path),
				Size: bytefmt.ByteSize(uint64(finfo.Size())),
				Time: fmt.Sprintf("%s %s", timearr[0][1:], strings.Split(timearr[1], ".")[0]),
			}
			if startTime.Format("2006-01-02") == timearr[0][1:] {
				thisfile.Today = true
			} else {
				thisfile.Today = false
			}
			fileList.List = append(fileList.List, thisfile)

		}
		return nil
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	const tpl = `<html>
	<head>
		<title>List</title>
		<style>
			table {border-collapse:collapse;}
			table td {border: 1px solid #afafaf;padding: 1px 3px;}
			table tr:hover {background-color:#f0f0ef}
			abbr {border-bottom:1px dotted black;cursor:help;}
			a {text-decoration:none;}
			a:hover {text-decoration:underline;}
			.now {background-color:#daffda;}
		</style>
	</head>
	<body>
	<table>
	{{range .List}}
	<tr><td><a href="{{.Name}}">{{.Name}}</a></td><td{{if .Today}} class="now"{{end}}>{{.Time}}</td><td>{{.Size}}</td></tr>
	{{end}}
	</table>
	<hr>
	<small><abbr title="GoLang Version">Go</abbr>: {{.Version}}</small> |
	<small><abbr title="Generation Time">GT</abbr>: {{.LoadTime}}</small> |
	<small><abbr title="Total Count">TC</abbr>: {{.TotalCount}}</small> |
	<small><abbr title="Total Size">TS</abbr>: {{.TotalSize}}</small>
	</body>
</html>`
	t, err := template.New("webpage").Parse(tpl)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fileList.TotalSize = bytefmt.ByteSize(totalSize)
	fileList.LoadTime = fmt.Sprintf("%q", time.Since(startTime))

	if err = t.Execute(w, fileList); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func robots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintln(w, "User-agent: *\nDisallow: /")
}

func main() {
	flag.Parse()
	fs := HideDir(http.FileServer(http.Dir(*osPath)))
	http.Handle(*webPath, http.StripPrefix(*webPath, fs))
	http.HandleFunc("/", index)
	http.HandleFunc("/list", BasicAuth(list, *login, *password))
	http.HandleFunc("/upload", Logger(BasicAuth(upload, *login, *password)))
	http.HandleFunc("/robots.txt", robots)
	bind := fmt.Sprintf("%s:%d", *httpAddr, *httpPort)
	log.Println("Starting on", bind)
	if *useFcgi {
		log.Println("Using FastCGI")
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
