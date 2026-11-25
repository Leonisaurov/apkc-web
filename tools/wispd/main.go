package main

import (
	"flag"
	"log"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
)

// El límite se fija aquí, no con GOMEMLIMIT: es un único mecanismo y no depende
// de cómo se arranque el proceso. debug.SetMemoryLimit devuelve el valor previo.
const limiteMemoria = 512 << 20

func fijarLimiteMemoria() int64 {
	return debug.SetMemoryLimit(limiteMemoria)
}

func main() {
	fijarLimiteMemoria()
	addr := flag.String("addr", "127.0.0.1:8800", "direccion de escucha")
	origenes := flag.String("origenes", "", "origenes adicionales separados por comas")
	cert := flag.String("cert", "", "certificado TLS; vacio sirve ws://")
	key := flag.String("key", "", "clave privada TLS; vacia sirve ws://")
	credito := flag.Uint("credito", 64, "credito inicial en paquetes del CONTINUE de streamID=0")
	maxStreams := flag.Uint("maxstreams", 8, "flujos maximos por conexion")
	dev := flag.Bool("dev", false, "permite tambien los origenes locales de desarrollo")
	flag.Parse()

	if (*cert == "") != (*key == "") {
		log.Fatal("-cert y -key deben ir juntos")
	}
	if *credito == 0 || *credito > 0xFFFFFFFF {
		log.Fatal("-credito debe estar entre 1 y 4294967295")
	}
	if *maxStreams < 1 {
		log.Fatal("-maxstreams debe ser al menos 1")
	}

	extras := make([]string, 0)
	for _, origen := range strings.Split(*origenes, ",") {
		if origen = strings.TrimSpace(origen); origen != "" {
			extras = append(extras, origen)
		}
	}
	allow, err := nuevaAllowlist(*dev, extras)
	if err != nil {
		log.Fatal(err)
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           nuevoServidor(allow, uint32(*credito), int(*maxStreams)),
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("wispd escuchando en %s", *addr)
	if *cert != "" {
		log.Fatal(srv.ListenAndServeTLS(*cert, *key))
	}
	log.Fatal(srv.ListenAndServe())
}
