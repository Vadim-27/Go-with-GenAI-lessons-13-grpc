// LibraTrack: невеликий доповнювач до HTMX (без фреймворків).
//
// За замовчуванням HTMX мовчки ігнорує відповіді з помилкою (401, 403, 404, 409).
// Тут ми показуємо їх у блоці #flash. Текст вставляється через textContent,
// а не innerHTML, тож відповідь сервера ніколи не виконується як HTML.
(function () {
  "use strict";

  function flash(message, isError) {
    var box = document.getElementById("flash");
    if (!box) return;
    box.textContent = message;
    box.classList.add("show");
    box.classList.toggle("error", !!isError);
  }

  document.addEventListener("htmx:responseError", function (evt) {
    var xhr = evt.detail.xhr;
    flash("Помилка " + xhr.status + ": " + (xhr.responseText || xhr.statusText), true);
  });

  document.addEventListener("htmx:sendError", function () {
    flash("Сервер недоступний", true);
  });

  // Успішне видалення: рядок уже зник (outerHTML + порожня відповідь), повідомляємо.
  document.addEventListener("htmx:afterRequest", function (evt) {
    var cfg = evt.detail.requestConfig;
    if (evt.detail.successful && cfg && cfg.verb === "delete") {
      flash("Книгу видалено", false);
    }
  });
})();
