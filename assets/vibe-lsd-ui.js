var plot;

function createTimeline(data) {
  const DATE_INDEX = 0;
  const COUNT_INDEX = 1;

  var csv = $.csv.toArrays(data, {
    onParseValue: $.csv.hooks.castToScalar
  });

  var parsed = csv.map(function(row) {
    var dateInMilis = row[DATE_INDEX] * 1000;
    return [new Date(dateInMilis).toUTCString(), row[COUNT_INDEX]];
  });

  if (plot) {
    plot.destroy();
  }

  plot = $.jqplot('timeline', [parsed], {
    gridPadding: {
      right: 35
    },
    cursor: {
      show: false
    },
    highlighter: {
      show: true,
      sizeAdjust: 6
    },
    axes: {
      xaxis: {
        renderer: $.jqplot.DateAxisRenderer,
        tickInterval: '1 week'
      },
      yaxis: {
        min: 0,
        label: ''
      }
    }
  });
}

function readFile(file) {
      var rawFile = new XMLHttpRequest();
      rawFile.open("GET", file, false);
      rawFile.onreadystatechange = function () {
        if (rawFile.readyState === 4) {
          if(rawFile.status === 200 || rawFile.status == 0) {
            var allText = rawFile.responseText;
            createTimeline(allText);
          }
        }
     }
     rawFile.send(null);
}

$(document).ready(function() {
  $('#metric')
    .change(function() {
      $('select option:selected').each(function() {
          var metricFile = $(this).text().toLowerCase() + '.csv';
          readFile(metricFile);
      });
    })
    .trigger('change');
});
