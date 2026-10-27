"use client";

export function RegistrationHelp() {
  return (
    <div
      id="registration-help-content"
      role="region"
      aria-label="Ayuda para encontrar el correo de verificación"
      className="mt-4 space-y-3 rounded-xl border border-gray-200 bg-gray-50 p-4 text-left text-sm text-gray-700"
    >
      <h3 className="font-semibold text-gray-900">
        ¿Qué puedes hacer si no encuentras el correo?
      </h3>
      <ul className="list-disc space-y-1.5 pl-5">
        <li>
          Revisa la carpeta de <strong>Spam</strong> o{" "}
          <strong>Correo no deseado</strong>. Si el mensaje está allí, márcalo
          como «No es spam».
        </li>
        <li>
          Espera unos minutos; algunos proveedores de correo tardan un momento
          en procesar mensajes entrantes.
        </li>
        <li>
          Verifica que la dirección ingresada no contenga errores tipográficos.
        </li>
        <li>
          Asegúrate de permitir correos de notificaciones de EspaciGo en tu
          cliente de correo.
        </li>
      </ul>
    </div>
  );
}

export default RegistrationHelp;
