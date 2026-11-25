package main

// Allowlist de destinos: es la capa que impide usar el relay como proxy abierto.
// El Origin del navegador solo decide quién puede hablarle al relay; aquí se
// decide a qué host:puerto puede hablar el relay. Son dos capas distintas.
//
// Los dos hosts salen de apkc/depswisp.go:90-93. El puerto es 443 porque TLS
// va de punta a punta entre la pestaña y el espejo de Maven Central.

import (
	"context"
	"net"
	"strconv"
	"strings"
)

const puertoDestino = 443

var destinosProduccion = map[string]uint16{
	"repo1.maven.org":       puertoDestino,
	"repo.maven.apache.org": puertoDestino,
}

// politicaDestinos separa la comprobación del nombre de la de la dirección
// resuelta. Así se puede inyectar una política de loopback en los tests sin
// añadir ninguna bandera que abra el relay en producción.
type politicaDestinos struct {
	host func(host string, puerto uint16, tipo byte) byte
	ip   func(ip net.IP, puerto uint16, tipo byte) byte
}

// direccionReservada cubre literal IP, loopback (127/8 y ::1), privadas
// (10/8, 172.16/12, 192.168/16 y fc00::/7) y link-local (169.254/16 y
// fe80::/10), además de multicast y la dirección no especificada.
func direccionReservada(ip net.IP) bool {
	return ip == nil || ip.IsUnspecified() || ip.IsLoopback() || ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast()
}

var politicaProduccion = politicaDestinos{
	host: func(host string, puerto uint16, tipo byte) byte {
		if puerto != puertoDestino {
			return motivoInfoInvalida
		}
		if tipo != tipoFlujoTCP {
			return motivoDestinoBloqueado
		}
		normalizado := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
		if normalizado == "" {
			return motivoInfoInvalida
		}
		if net.ParseIP(normalizado) != nil {
			return motivoInfoInvalida
		}
		if _, ok := destinosProduccion[normalizado]; !ok {
			return motivoDestinoBloqueado
		}
		return 0
	},
	ip: func(ip net.IP, puerto uint16, tipo byte) byte {
		if direccionReservada(ip) {
			return motivoInfoInvalida
		}
		// El nombre y el puerto ya pasaron la allowlist; aquí solo se descartan
		// direcciones reservadas. Bloquear también las públicas impediría los dos
		// CONNECT legítimos a los espejos de Maven Central.
		return 0
	},
}

// resolverDestino valida antes de abrir el socket y devuelve la dirección ya
// resuelta, para que un cambio de DNS entre la validación y el dial no pueda
// colar una IP privada (rebinding).
func (s *servidor) resolverDestino(ctx context.Context, host string, puerto uint16, tipo byte) (string, byte) {
	if motivo := s.destinos.host(host, puerto, tipo); motivo != 0 {
		return "", motivo
	}
	ips, err := s.lookupIP(ctx, host)
	if err != nil || len(ips) == 0 {
		return "", motivoHostInalcanzable
	}
	for _, direccion := range ips {
		if motivo := s.destinos.ip(direccion.IP, puerto, tipo); motivo != 0 {
			return "", motivo
		}
	}
	return net.JoinHostPort(ips[0].IP.String(), strconv.Itoa(int(puerto))), 0
}
