import moment from 'moment-timezone';

import './sass/styles.scss';

function dayNameFromDayIndex(idx) {
  switch (idx) {
      case 0:
          return "Sunday";
      case 1:
          return "Monday";
      case 2:
          return "Tuesday";
      case 3:
          return "Wednesday";
      case 4:
          return "Thursday";
      case 5:
          return "Friday";
      case 6:
          return "Saturday";
  }
}

var number;

window.onload = () => {
  document.getElementById('current-day-label').innerHTML = 'Today is <span>' +
    dayNameFromDayIndex(new Date().getDay()) + '!</span>';

  var numberInput = document.getElementById('number');
  number = intlTelInput(numberInput, {
      placeholderNumberType: 'MOBILE',
  });

  var tz = moment.tz.guess(true);

  if (tz) {
      var timezoneSelect = document.getElementById('timezone');
      timezoneSelect.value = tz;
  }
};


$('#subscription').on('submit', function() {
  $('#subscribe-form').addClass('hidden');
  $('#progress-indicator').removeClass('hidden');

  const timezone = $('#timezone').val();

  const payload = {
      timezone: timezone,
      number: number.getNumber(),
  };

  $.post({
      url: '/api/subscribe',
      dataType: 'json',
      contentType: 'application/json',
      data: JSON.stringify(payload)
  }).done(function(data) {
      $('.circle-loader').addClass('load-complete');
      $('.checkmark').show();
  }).fail(function(data) {
      $('#subscribe-form').removeClass('hidden');
      $('#progress-indicator').addClass('hidden');
  });

  return false;
});