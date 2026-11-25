package main

import (
	"fmt"
	"net/url"
	"strings"
)

// La allowlist es de origen de navegador, no de servidor: wisp.dolci-vita.link
// es la máquina que sirve el relay y no debe aparecer aquí.
const origenProduccion = "https://apkc-studio.surge.sh"

var origenesDesarrollo = []string{
	"http://127.0.0.1:8801",
	"http://127.0.0.1:8787",
	"http://127.0.0.1:8791",
}

type allowlist struct {
	permitidos map[string]struct{}
}

// nuevaAllowlist fija el origen de producción y, solo con -dev, los tres
// orígenes locales. -origenes añade orígenes explícitos del operador.
func nuevaAllowlist(dev bool, extras []string) (*allowlist, error) {
	a := &allowlist{permitidos: map[string]struct{}{origenProduccion: {}}}
	if dev {
		for _, origen := range origenesDesarrollo {
			a.permitidos[origen] = struct{}{}
		}
	}
	for _, origen := range extras {
		normalizado, err := normalizarOrigen(origen)
		if err != nil {
			return nil, fmt.Errorf("origen adicional %q: %w", origen, err)
		}
		a.permitidos[normalizado] = struct{}{}
	}
	return a, nil
}

// normalizarOrigen compara esquema y host en minúsculas. El Origin del
// navegador no lleva barra final; se acepta de todos modos por si un cliente
// la añade, pero nunca query, fragmento, usuario ni una ruta concreta.
func normalizarOrigen(origen string) (string, error) {
	if strings.TrimSpace(origen) == "" || strings.EqualFold(strings.TrimSpace(origen), "null") {
		return "", fmt.Errorf("origen vacío o nulo")
	}
	u, err := url.Parse(origen)
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("esquema no permitido %q", u.Scheme)
	}
	if u.Host == "" || u.Opaque != "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("origen con partes no permitidas")
	}
	if u.Path != "" && u.Path != "/" {
		return "", fmt.Errorf("origen con ruta %q", u.Path)
	}
	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host), nil
}

func (a *allowlist) permite(origen string) bool {
	normalizado, err := normalizarOrigen(origen)
	if err != nil {
		return false
	}
	_, ok := a.permitidos[normalizado]
	return ok
}
