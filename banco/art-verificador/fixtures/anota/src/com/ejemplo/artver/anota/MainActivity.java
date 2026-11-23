package com.ejemplo.artver.anota;

import android.app.Activity;
import android.os.Bundle;
import android.util.Log;
import android.widget.TextView;

import java.lang.reflect.Field;
import java.lang.reflect.Method;
import java.lang.reflect.ParameterizedType;
import java.lang.reflect.Type;
import java.util.ArrayList;
import java.util.List;

// El bloque de anotaciones del dex: `encoded_annotation` con un elemento `enum`, uno entero y uno
// de cadena en clase, campo y método, más la anotación `Signature` que javac deja en un campo
// genérico. Se lee por reflexión porque es ART quien decodifica: si el bloque está mal, `dexdump`
// lo imprime igual y la lectura revienta o devuelve basura.
@Etiqueta(color = Color.AZUL, flags = 5, firma = "de clase")
public class MainActivity extends Activity {

    @Etiqueta(color = Color.VERDE, flags = 6, firma = "de campo")
    public List<String> lista = new ArrayList<String>();

    @Etiqueta(color = Color.AZUL, flags = 9, firma = "\u00f1and\u00fa")
    public String metodo(String x) {
        return x;
    }

    @Override
    public void onCreate(Bundle estado) {
        super.onCreate(estado);
        String salida;
        try {
            Etiqueta deClase = getClass().getAnnotation(Etiqueta.class);
            Field campo = getClass().getField("lista");
            Etiqueta deCampo = campo.getAnnotation(Etiqueta.class);
            Method metodo = getClass().getMethod("metodo", String.class);
            Etiqueta deMetodo = metodo.getAnnotation(Etiqueta.class);
            Type generico = campo.getGenericType();
            String argumento = "?";
            if (generico instanceof ParameterizedType) {
                argumento = ((ParameterizedType) generico).getActualTypeArguments()[0].toString();
            }
            salida = "clase=" + deClase.color() + "/" + deClase.flags() + "/" + deClase.firma()
                + " campo=" + deCampo.color() + "/" + deCampo.flags() + "/" + deCampo.firma()
                + " metodo=" + deMetodo.color() + "/" + deMetodo.flags() + "/" + deMetodo.firma()
                + " generico=" + argumento;
        } catch (Throwable e) {
            salida = "excepcion=" + e.getClass().getName() + ":" + e.getMessage();
        }
        Log.d("artver", "RESULTADO " + salida);
        TextView texto = new TextView(this);
        texto.setText(salida);
        setContentView(texto);
    }
}
