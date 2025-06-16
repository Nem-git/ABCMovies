
<head>
  <link href="./video-js.min.css" rel="stylesheet" />
  <script src="./video.min.js.js">
    let player = videojs("video");
    player.play()
    </script>

  <!-- If you'd like to support IE8 (for Video.js versions prior to v7) -->
  <!-- <script src="https://vjs.zencdn.net/ie8/1.1.2/videojs-ie8.min.js"></script> -->
</head>

<body>

<video id="video" width=600 height=300 class="video-js vjs-default-skin" controls>
    <source src="https://dash.akamaized.net/dash264/TestCasesUHD/2b/11/MultiRate.mpd" type="application/dash+xml">
</video>



  
</body>