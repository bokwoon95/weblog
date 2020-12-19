document.addEventListener("DOMContentLoaded", function() {
  const globals = document.querySelectorAll(".contenteditable-global");
  for (element of globals) {
    element.setAttribute("contenteditable", "true");
  }
  const saveBtn = document.querySelector("#save");
  if (saveBtn) {
    saveBtn.addEventListener("click", async function(e) {
      let keyValuePairs = [];
      for (element of globals) {
        keyValuePairs.push({ key: element.getAttribute("id"), value: element.innerHTML });
      }
      try {
        const resp = await fetch('/pm-kv', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ key_value_pairs: keyValuePairs, redirect_to: window.location.pathname, }),
        });
        const text = await resp.text();
        console.log(text);
      } catch(err) {
        console.log(err);
      }
    });
  }
});
