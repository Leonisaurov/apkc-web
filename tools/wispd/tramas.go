package main

// Formato de paquete del protocolo WISP v1.2. La referencia de tramas que ya
// usa el cliente de este repo está en apkc/wisp.go; aquí se reproduce el mismo
// formato, sin compartir código con el wasm:
//
//	WISP v1.2, Copyright (c) ading2210, CC-BY-4.0
//	https://github.com/MercuryWorkshop/wisp-protocol
//
// El paquete es `[tipo u8][streamID u32 LE][carga...]`. DATA no lleva longitud
// y se come el resto del mensaje del WebSocket, así que va siempre al final;
// los demás tipos tienen carga de longitud conocida y pueden ir coalescidos.

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	tipoConnect  byte = 0x01
	tipoData     byte = 0x02
	tipoContinue byte = 0x03
	tipoClose    byte = 0x04

	tipoFlujoTCP byte = 0x01
	tipoFlujoUDP byte = 0x02

	// Motivos de cierre de la tabla de Wisp v1.2 (URL verificada en el README).
	// 0x41-0x49 son exclusivos del servidor; 0x01-0x03 son compartidos.
	motivoCierreGenerico       byte = 0x01
	motivoCierreVoluntario     byte = 0x02
	motivoErrorRed             byte = 0x03
	motivoInfoInvalida         byte = 0x41
	motivoHostInalcanzable     byte = 0x42
	motivoTiempoAgotado        byte = 0x43
	motivoConexionRechazada    byte = 0x44
	motivoTransferenciaAgotada byte = 0x47
	motivoDestinoBloqueado     byte = 0x48
	motivoLimitado             byte = 0x49

	cabeceraPaquete = 5
	tamMaxDatos     = 16 << 10
	tamMaxHost      = 253
)

var (
	errPaqueteCorto     = errors.New("wispd: paquete WISP demasiado corto")
	errTipoDesconocido  = errors.New("wispd: tipo de paquete WISP desconocido")
	errConnectCorto     = errors.New("wispd: CONNECT sin tipo de flujo, puerto o nombre")
	errHostVacio        = errors.New("wispd: CONNECT con nombre de host vacío o no válido")
	errFlujoDesconocido = errors.New("wispd: tipo de flujo no soportado")
)

type paquete struct {
	tipo     byte
	streamID uint32
	carga    []byte
}

type solicitudConnect struct {
	tipoFlujo byte
	puerto    uint16
	host      string
}

func codificarPaquete(tipo byte, id uint32, carga []byte) []byte {
	out := make([]byte, cabeceraPaquete+len(carga))
	out[0] = tipo
	binary.LittleEndian.PutUint32(out[1:cabeceraPaquete], id)
	copy(out[cabeceraPaquete:], carga)
	return out
}

// decodificarPaquete devuelve el primer paquete del mensaje y lo que queda
// después de él. Un mensaje puede traer varios paquetes seguidos.
func decodificarPaquete(mensaje []byte) (paquete, []byte, error) {
	if len(mensaje) < cabeceraPaquete {
		return paquete{}, nil, errPaqueteCorto
	}
	p := paquete{
		tipo:     mensaje[0],
		streamID: binary.LittleEndian.Uint32(mensaje[1:cabeceraPaquete]),
	}
	resto := mensaje[cabeceraPaquete:]
	switch p.tipo {
	case tipoData:
		p.carga = resto
		return p, nil, nil
	case tipoConnect:
		// El hostname ocupa hasta el final del mensaje.
		p.carga = resto
		return p, nil, nil
	case tipoContinue:
		if len(resto) < 4 {
			return paquete{}, nil, errPaqueteCorto
		}
		p.carga = resto[:4]
		return p, resto[4:], nil
	case tipoClose:
		if len(resto) < 1 {
			return paquete{}, nil, errPaqueteCorto
		}
		p.carga = resto[:1]
		return p, resto[1:], nil
	default:
		return paquete{}, nil, fmt.Errorf("%w: 0x%02x", errTipoDesconocido, p.tipo)
	}
}

func decodificarConnect(carga []byte) (solicitudConnect, error) {
	if len(carga) < 3 {
		return solicitudConnect{}, errConnectCorto
	}
	sol := solicitudConnect{
		tipoFlujo: carga[0],
		puerto:    binary.LittleEndian.Uint16(carga[1:3]),
		host:      string(carga[3:]),
	}
	if sol.tipoFlujo != tipoFlujoTCP && sol.tipoFlujo != tipoFlujoUDP {
		return solicitudConnect{}, errFlujoDesconocido
	}
	if sol.host == "" || len(sol.host) > tamMaxHost || !utf8.ValidString(sol.host) ||
		strings.ContainsRune(sol.host, 0) || strings.ContainsAny(sol.host, "\r\n") {
		return solicitudConnect{}, errHostVacio
	}
	return sol, nil
}

func creditoDe(p paquete) uint32 {
	if p.tipo != tipoContinue || len(p.carga) < 4 {
		return 0
	}
	return binary.LittleEndian.Uint32(p.carga[:4])
}

func motivoDe(p paquete) byte {
	if p.tipo != tipoClose || len(p.carga) < 1 {
		return motivoCierreGenerico
	}
	return p.carga[0]
}
