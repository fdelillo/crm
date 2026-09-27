# ADR-007: Hashing de contraseñas con argon2id

**Status**: Proposed
**Fecha**: 2026-09-27
**Origen**: spec 001 (FR-003)

## Contexto

FR-003 exige guardar las contraseñas con un hash robusto (argon2id o bcrypt). El servidor corre
como una sola instancia con memoria acotada, y el login es un endpoint público que puede recibir
ráfagas.

## Decisión

- **argon2id** con `golang.org/x/crypto/argon2` (`IDKey`), parámetros mínimos de OWASP:
  memoria 19 MiB (`m=19456` KiB), 2 iteraciones, paralelismo 1, sal aleatoria de 16 bytes, clave
  de 32 bytes.
- Formato **PHC** en `users.password_hash`:
  `$argon2id$v=19$m=19456,t=2,p=1$<sal b64>$<hash b64>`. Los parámetros viajan con el hash.
- **Rehash** transparente: si en un login correcto los parámetros guardados son más débiles que
  los vigentes, se recalcula y se guarda.
- **Semáforo** de concurrencia (default 4, configurable) alrededor de `Hash` y `Verify`: una
  ráfaga de logins no puede reservar más de 4 × 19 MiB a la vez.
- Comparación en tiempo constante (`crypto/subtle`).
- `VerifyDummy`: verificación contra un hash ficticio con los mismos parámetros, para que un email
  inexistente cueste lo mismo que uno existente (anti-enumeración por tiempo).
- Contraseñas de 10 a 128 caracteres, sin reglas de composición, distintas del email.

## Fundamento

- OWASP recomienda argon2id en primer lugar; su costo de memoria encarece los ataques con GPU.
- El formato PHC permite subir parámetros en el futuro sin migrar a todos los usuarios de golpe.
- El semáforo convierte un riesgo de agotamiento de memoria en una cola con latencia acotada.
- 128 caracteres como máximo acota el costo por intento sin afectar a usuarios reales.

## Alternativas consideradas

- **bcrypt**: admitido por FR-003 y más simple (sin parámetro de memoria), pero trunca la entrada
  a 72 bytes y es menos resistente a hardware dedicado. Queda como alternativa si la memoria del
  hosting fuera muy escasa.
- **scrypt / PBKDF2**: OWASP los ubica por detrás de argon2id.
- **Reglas de composición** (mayúscula, número, símbolo): las guías actuales las desaconsejan
  porque empujan a patrones predecibles; se prefiere largo mínimo.

## Consecuencias

- Ganás: el algoritmo recomendado y un camino para endurecer parámetros sin migraciones.
- Aceptás: 19 MiB por verificación en curso y un límite de concurrencia que, bajo ataque, agrega
  latencia al login (mitigado por el rate limit por IP). Un mínimo de 10 caracteres está por
  debajo de lo que algunas guías recientes piden para contraseñas de un solo factor: se prioriza
  la carga desde el celular y se puede subir sin cambiar el esquema.
