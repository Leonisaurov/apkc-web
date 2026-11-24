#!/usr/bin/env node
// falsificación del árbol roto tras vaciar `build/res`: el shim borraba el nodo del directorio
// desde `unlink`, así que `os.RemoveAll("build/res")` decía haber hecho su trabajo dejando los
// hijos huérfanos en el mapa. `Stat("build/res/layout")` seguía contestando que sí, `MkdirAll`
// no lo recreaba y el `open` del recurso fallaba con ENOENT: el enlace de recursos de apkc se
// llevaba por delante el binario de pruebas (LogF hace os.Exit) y solo corrían 118 de 847.
// Este guion compila una sonda fuera del repo y la corre por el shim: exige el árbol coherente
// y que `unlink` de un directorio conteste EISDIR, como Linux.
//
// uso: node shim/prueba-relink.js        (necesita go y shim/wasm_exec.js de tu GOROOT)
//      SHIM=<copia>/navegador-sim.js node shim/prueba-relink.js   para correrlo contra otro shim
const fs = require("fs");
const os = require("os");
const path = require("path");
const { spawnSync } = require("child_process");

const dirShim = __dirname;
const shim = process.env.SHIM ? path.resolve(process.env.SHIM) : path.join(dirShim, "navegador-sim.js");
const cache = process.env.XDG_CACHE_HOME
  ? path.join(process.env.XDG_CACHE_HOME, "apkc-web")
  : path.join(os.homedir(), ".cache", "apkc-web");
const tmp = path.join(cache, "prueba-relink");

if (!fs.existsSync(path.join(dirShim, "wasm_exec.js"))) {
  console.error("falta shim/wasm_exec.js: cp \"$(go env GOROOT)/lib/wasm/wasm_exec.js\" shim/");
  process.exit(1);
}

// la sonda repite lo que hace el enlazador: escribe un recurso, vacía `build/res`, rehace el
// directorio y vuelve a escribir; y comprueba aparte la semántica de unlink/rmdir de Linux
const SONDA = `package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func main() {
	raiz, err := os.MkdirTemp("/tmp", "relink")
	if err != nil {
		fmt.Println("sin tmp:", err)
		os.Exit(2)
	}
	layout := filepath.Join(raiz, "build", "res", "layout")
	if err := os.MkdirAll(layout, 0o755); err != nil {
		fmt.Println("mkdirall:", err)
		os.Exit(2)
	}
	if err := os.WriteFile(filepath.Join(layout, "x.xml"), []byte("hola"), 0o644); err != nil {
		fmt.Println("write inicial:", err)
		os.Exit(2)
	}
	if err := os.RemoveAll(filepath.Join(raiz, "build", "res")); err != nil {
		fmt.Println("removeall:", err)
		os.Exit(2)
	}
	if _, err := os.Stat(layout); !os.IsNotExist(err) {
		fmt.Println("huerfano: stat de", layout, "tras RemoveAll ->", err)
		os.Exit(2)
	}
	if err := os.MkdirAll(layout, 0o755); err != nil {
		fmt.Println("mkdirall tras RemoveAll:", err)
		os.Exit(2)
	}
	if err := os.WriteFile(filepath.Join(layout, "x.xml"), []byte("adios"), 0o644); err != nil {
		fmt.Println("write tras RemoveAll:", err)
		os.Exit(2)
	}
	datos, err := os.ReadFile(filepath.Join(layout, "x.xml"))
	if err != nil || string(datos) != "adios" {
		fmt.Println("releer tras RemoveAll:", err)
		os.Exit(2)
	}
	fmt.Println("arbol=coherente")

	// unlink sobre un directorio es EISDIR (Linux); si no, os.Remove no cae a rmdir
	if err := syscall.Unlink(raiz); err != syscall.EISDIR {
		fmt.Println("unlink de directorio =", err, "(se espera EISDIR)")
		os.Exit(2)
	}
	// y rmdir de uno que aún tiene algo es ENOTEMPTY, que es lo que hace recorrer a RemoveAll
	if err := syscall.Rmdir(filepath.Join(raiz, "build")); err != syscall.ENOTEMPTY {
		fmt.Println("rmdir no vacio =", err, "(se espera ENOTEMPTY)")
		os.Exit(2)
	}
	fmt.Println("semantica=linux")
	os.Exit(0)
}
`;

const entorno = { ...process.env, GOOS: "js", GOARCH: "wasm", CGO_ENABLED: "0", GOTMPDIR: path.join(tmp, "gotmp") };
fs.mkdirSync(entorno.GOTMPDIR, { recursive: true });
if (spawnSync("go", ["version"], { encoding: "utf8" }).status !== 0) {
  console.error("esta prueba compila su sonda con go, y go no está en el PATH");
  process.exit(1);
}

const dirSonda = path.join(tmp, "sonda");
fs.mkdirSync(dirSonda, { recursive: true });
fs.writeFileSync(path.join(dirSonda, "main.go"), SONDA);
const wasm = path.join(tmp, "sonda.wasm");
const compilado = spawnSync("go", ["build", "-o", wasm, "main.go"], { cwd: dirSonda, env: entorno, encoding: "utf8", timeout: 180000 });
if (compilado.status !== 0) {
  console.error("no compilé la sonda: " + ((compilado.stderr || "") + (compilado.stdout || "")));
  process.exit(1);
}

const corrida = spawnSync(process.execPath, [shim, wasm], { cwd: dirShim, encoding: "utf8", timeout: 60000 });
const salida = (corrida.stdout || "") + (corrida.stderr || "");
const fallos = [];
function comprobar(nombre, condicion, detalle) {
  if (condicion) {
    console.log(`  ok   ${nombre}`);
  } else {
    fallos.push(nombre);
    console.log(`  FALLO ${nombre}${detalle ? " — " + detalle : ""}`);
  }
}

console.log("1. RemoveAll de build/res + MkdirAll + WriteFile tienen que dejar el recurso escrito");
comprobar("rc 0", corrida.status === 0, `rc ${corrida.status}, salida: ${salida.trim().slice(0, 300)}`);
comprobar("sin huérfanos tras RemoveAll", salida.includes("arbol=coherente"), salida.trim().slice(0, 300));
comprobar("el recurso reescrito se relee", salida.includes("arbol=coherente"));

console.log("2. unlink/rmdir con la semántica de Linux");
comprobar("unlink de directorio = EISDIR y rmdir no vacío = ENOTEMPTY", salida.includes("semantica=linux"), salida.trim().slice(0, 300));

if (fallos.length) {
  console.error(`\nprueba-relink: ${fallos.length} fallo(s): ${fallos.join(", ")}`);
  process.exit(1);
}
console.log("\nprueba-relink: RemoveAll deja el árbol coherente y unlink respeta EISDIR");
