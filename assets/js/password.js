(function () {
  var form = document.getElementById('password-form'), error = document.getElementById('password-error');
  form.onsubmit = async function (event) {
    event.preventDefault(); error.hidden = true;
    var button = form.querySelector('button'); button.disabled = true;
    try {
      var response = await fetch('/admin/password', {method:'POST',headers:{'Accept':'application/json'},body:new URLSearchParams(new FormData(form)),redirect:'follow'});
      if (response.redirected && response.ok) {window.location.assign('/login');return;}
      var result = await response.json();
      throw new Error(result.error || 'Could not change your password. Please try again.');
    } catch (e) {error.textContent=e.message;error.hidden=false;button.disabled=false;}
  };
}());
