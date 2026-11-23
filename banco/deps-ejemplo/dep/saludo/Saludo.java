package saludo;

// Clase de la dependencia local: construir.sh la compila con el JDK y la empaqueta en
// jar/saludo.jar. No viene de la red ni de Maven; el jar es un artefacto del propio banco.
public final class Saludo {
    private Saludo() {
    }

    public static String saludo() {
        return "hola desde el jar";
    }
}
