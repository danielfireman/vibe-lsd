function createTimeline() {
  var data = [['2008-06-30 8:00AM',4], ['2008-7-14 8:00AM',6.5], ['2008-7-28 8:00AM',5.7], ['2008-8-11 8:00AM',9], ['2008-8-25 8:00AM',8.2]];
  var plot = $.jqplot('timeline', [data], {
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
        tickInterval: '2 weeks'
      },
      yaxis: {
        min: 0,
        label: ''
      }
    }
  });
}

$(document).ready(function() {
  createTimeline();
});
