function createTimeline(data) {
  var csv = $.csv.toArrays(data, {
    onParseValue: $.csv.hooks.castToScalar
  });

  var parsed = csv.map(function(row) {
    return [new Date(row[0]).toUTCString(), row[1]];
  });

  var plot = $.jqplot('timeline', [parsed], {
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
  readFile('./commits.csv');
});
