package main

import (
	"crypto/md5"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/fcgi"
	"os"
	"strings"
)

var err error

var (
	HttpAddr = flag.String("address", "127.0.0.1", "Http address")
	HttpPort = flag.Int("port", 8080, "Http port")
	WebPath  = flag.String("webpath", "/storage/", "Web Path")
	OsPath   = flag.String("ospath", "./storage", "OS Path")
	Login    = flag.String("login", "admin", "Web login")
	Password = flag.String("password", "admin123", "Web password")
	Usefcgi  = flag.Bool("fcgi", false, "FastCGI")
)

func ifErr(err error) {
	if err != nil {
		log.Panicln(err.Error())
	}
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
		http.Error(w, err.Error(), 500)
		return
	}
	defer infile.Close()

	bs, err := ioutil.ReadAll(infile)
	if err != nil {
		http.Error(w, err.Error(), 500)
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

	filepath := fmt.Sprintf("%s/%x/%x/%x%s", *OsPath, hashed[0:1], hashed[1:2], hashed[2:], ext)

	err = os.MkdirAll(fmt.Sprintf("%s/%x/%x/", *OsPath, hashed[0:1], hashed[1:2]), 0755)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	err = ioutil.WriteFile(filepath, bs, 0644)

	path := fmt.Sprintf("%s/%x/%x/%x%s", *WebPath, hashed[0:1], hashed[1:2], hashed[2:], ext)
	http.Redirect(w, r, path, 302)
}

func index(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Hello!")
}

func main() {
	flag.Parse()
	fs := HideDir(http.FileServer(http.Dir(*OsPath)))
	http.Handle(*WebPath, http.StripPrefix(*WebPath, fs))
	http.HandleFunc("/", index)
	http.HandleFunc("/upload", logger(BasicAuth(upload, *Login, *Password)))
	bind := fmt.Sprintf("%s:%d", *HttpAddr, *HttpPort)
	log.Println("Starting on", bind)
	if *Usefcgi {
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
