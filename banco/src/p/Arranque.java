package p;

// El banco se ejecuta con `app_process` en el aparato, que exige un `main(String[])`. Este fork
// todavia no emite tipos de array (parrafo 5, item 5.11 del plan), asi que esta clase la compila
// javac y la nuestra (`p.Ver`) la escribe `apkc compile`: se dexan las dos con `apkc dex` y el
// resultado se compara con el de la JVM sobre las mismas fuentes. Sin `+` de cadenas aqui: javac 21 lo
// baja con `invokedynamic` y el dexer lo rechaza por nombre (plan 5.22).
public class Arranque {
    public static void main(String[] args) {
        Ver.cuenta();
        Ver.cuenta();
        Ver.suma(3);
        Ver.elige(1);
        Ver.cuenta();
        int t = Ver.total;
        System.exit(t);
    }
}
