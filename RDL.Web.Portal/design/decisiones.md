# Decisiones abiertas y supuestos

1. **Códigos fiscales ilustrativos.** CABYS, tarifas de impuesto («13 %», «4 %», «Exento»), medios de pago, clave numérica, consecutivo, motivos de rechazo y mensajes de Hacienda son de muestra. Catálogo oficial pendiente.
2. **Reglas de Hacienda no confirmadas.** Textos de contingencia («se reenviará automáticamente»), reintento sin duplicar y corrección de rechazos con nota de crédito deben validarse con un experto en facturación electrónica de Costa Rica.
3. **Tramos del aging.** Al día, 1–30, 31–60, 61–90, más de 90. Se proponen configurables por organización; falta definir quién los cambia.
4. **Exportación de auditoría.** Depende de una API no definida. Columnas propuestas: fecha y hora (hora de Costa Rica), usuario, rol, acción, documento o recurso, detalle, resultado.
5. **Decimales.** Montos con 2 decimales; cantidades con hasta 3. Confirmar.
6. **Tipo de cambio.** Se muestra editable con valor ilustrativo; falta definir la fuente oficial.
7. **Invitaciones.** No se envía correo; el enlace se muestra una sola vez y vence en 7 días (supuesto).
8. **Iconos.** Glifos provisionales en estados; iconos del menú en estilo Lucide. Se propone adoptar Lucide completo.
9. **Tema oscuro.** Definido en tokens. En el prototipo se simula con una capa de estilos; en producción debe salir de las variables.
10. **Marca.** «RDL» y su icono (opción 1a) son provisionales y fáciles de reemplazar.
11. **Densidad por defecto.** Cómoda (52 px); compacta (36 px) como preferencia del usuario (supuesto: se guarda por usuario).
12. **Notificaciones en tiempo real.** Reconexión cada 8 s con cuenta regresiva (supuesto).
13. **Pantallas propuestas fuera del brief.** Ninguna; el formulario «Editar cliente con cambios sin guardar» del prototipo ilustra la pantalla 8.
14. **Revisión pendiente.** Falta la verificación pantalla por pantalla a 390 px y 1024 px.
