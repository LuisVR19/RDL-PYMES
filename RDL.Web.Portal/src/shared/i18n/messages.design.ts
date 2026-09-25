// Generado por scripts/import-texts.mjs desde design/textos.csv. No editar a mano.
export const designMessages = {
  'app.env.test': 'Ambiente de pruebas',
  'app.env.prod': 'Producción',
  'app.search.placeholder': 'Buscar facturas, clientes, CABYS…',
  'app.theme.toDark': 'Cambiar a tema oscuro',
  'app.theme.toLight': 'Cambiar a tema claro',
  'auth.login.title': 'Iniciar sesión',
  'auth.login.subtitle': 'Ingrese con su correo y contraseña.',
  'auth.login.submit': 'Ingresar',
  'auth.login.submitting': 'Ingresando…',
  'auth.login.error': 'El correo o la contraseña no son correctos. Revíselos e intente de nuevo.',
  'auth.recover.title': 'Recuperar contraseña',
  'auth.recover.sent':
    'Si el correo está registrado, recibirá un enlace en unos minutos. El enlace vence en 30 minutos.',
  'auth.session.expired':
    'Por seguridad, ingrese de nuevo su contraseña. Seguirá en la misma página y no perderá lo que estaba viendo.',
  'org.select.title': 'Elija una organización',
  'org.select.empty': 'Todavía no pertenece a ninguna organización',
  'org.switch.dirty': 'Si cambia a {org}, perderá los cambios sin guardar de esta página.',
  'org.switch.done': 'Ahora trabaja en {org}',
  'common.cancel': 'Cancelar',
  'common.save': 'Guardar',
  'common.retry': 'Reintentar',
  'common.refCode': 'Código de referencia: {code}',
  'common.copy': 'Copiar',
  'common.prev': '‹ Anterior',
  'common.next': 'Siguiente ›',
  'common.unsaved': 'Cambios sin guardar',
  'common.partialData':
    'Algunos datos no están disponibles en este momento; el resto de la información está al día.',
  'common.reasonRequired': 'Escriba el motivo (mínimo 10 caracteres).',
  'client.editWarning': 'Editar un cliente no cambia las facturas ya emitidas.',
  'client.duplicate': 'Ya existe un cliente con esta identificación: {name}.',
  'invoice.new': 'Nueva factura',
  'invoice.saveDraft': 'Guardar borrador',
  'invoice.emit': 'Emitir factura…',
  'invoice.emit.title': '¿Emitir la factura?',
  'invoice.emit.warning':
    'Al emitirla recibe número y ya no se puede editar. Para corregirla después habrá que crear una nota de crédito.',
  'invoice.emit.error':
    'No pudimos emitir la factura. Sigue como borrador y no se asignó número; puede reintentar sin riesgo de duplicarla.',
  'invoice.issued': 'Factura {n} emitida',
  'invoice.issuedSub': 'Hacienda: En proceso. Le avisaremos cuando responda.',
  'invoice.alreadyIssued': 'La factura ya fue emitida; para corregirla cree una nota de crédito.',
  'invoice.discountReason': 'El motivo es obligatorio cuando hay descuento.',
  'invoice.totals.busy': 'Recalculando…',
  'invoice.totals.ok': 'Calculado por el sistema a las {time}',
  'invoice.totals.error':
    'No pudimos recalcular los totales. Los montos son del último cálculo y no son definitivos.',
  'invoice.void.title': 'Anular {n}',
  'invoice.void.body':
    'La factura queda anulada y su cuenta por cobrar se ajusta: el saldo de {saldo} pasa a {cero}. Esta acción no se puede deshacer.',
  'hacienda.processing': 'Hacienda está procesando el documento. Le avisaremos cuando responda.',
  'hacienda.rejected': 'Hacienda rechazó el documento',
  'hacienda.contingency':
    'El documento quedó en contingencia y se enviará de nuevo automáticamente. No necesita hacer nada.',
  'hacienda.unavailable': 'No pudimos obtener el estado de Hacienda; el resto de la información está al día.',
  'hacienda.envProd.confirm': 'Escriba PRODUCCIÓN para confirmar',
  'hacienda.cert.expiring':
    'Su certificado de firma vence el {date} (en {days} días). Súbalo de nuevo antes de esa fecha para no interrumpir la facturación.',
  'hacienda.cert.none': 'No puede emitir documentos: falta el certificado de firma.',
  'hacienda.cert.pin': 'Se guarda cifrado y no se vuelve a mostrar en el portal.',
  'hacienda.inbox.empty': 'Cuando Hacienda rechace un documento o no lo pueda recibir aparecerá aquí.',
  'ar.payment.register': 'Registrar pago',
  'ar.payment.over': 'Lo aplicado ({applied}) supera el pago por {diff}. Reduzca alguna aplicación.',
  'ar.payment.rowOver': 'Supera el saldo de esta cuenta ({saldo}).',
  'ar.payment.unapplied':
    'Quedarán {amount} sin aplicar. Podrá aplicarlos después desde el detalle del pago.',
  'ar.payment.registered': 'Pago {n} registrado',
  'ar.revert.title': 'Revertir aplicación',
  'ar.void.body':
    'El pago quedará anulado y se revertirán todas sus aplicaciones. Esta acción no se puede deshacer.',
  'admin.users.ownerRule':
    'La organización debe tener siempre al menos un propietario. Solo un propietario puede asignar o quitar el rol de propietario.',
  'admin.users.lastOwner':
    'La organización debe tener al menos un propietario. Asigne otro propietario antes de cambiar este rol.',
  'admin.invite.linkOnce':
    'Este enlace solo se muestra ahora. Cópielo y compártalo con la persona invitada; vence en 7 días.',
  'admin.invite.noEmail':
    'Hoy el portal no envía correos: al crear la invitación verá un enlace para compartirlo a mano.',
  'admin.branch.codeRule': 'El código de cada sucursal es único y no se puede cambiar.',
  'sys.403.title': 'No tiene acceso a esta sección',
  'sys.403.body':
    'Su rol en {org} es {role}. Si necesita esta sección, pida acceso a un administrador o al propietario.',
  'sys.404.title': 'No encontramos esta página',
  'sys.404.body': 'Puede que el enlace esté incompleto o que el documento no exista en esta organización.',
  'sys.unavailable.title': 'El servicio no está disponible en este momento',
  'sys.error.title': 'Algo salió mal',
  'rt.offline':
    'Sin conexión en tiempo real. Las notificaciones pueden llegar con retraso; el resto del portal funciona.',
} as const
