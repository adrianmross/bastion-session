Output docs/assets/bastion-session-demo.gif

Require bash

Set Shell "bash"
Set FontSize 22
Set FontFamily "Menlo"
Set LetterSpacing 0
Set LineHeight 1.2
Set Width 1180
Set Height 680
Set Theme "TokyoNight"
Set Padding 18
Set WindowBar Colorful
Set BorderRadius 8
Set TypingSpeed 35ms
Set Framerate 30

Hide
Type "export PATH=./docs/demo/bin:/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin"
Enter
Type "export PS1='$ '"
Enter
Type "clear"
Enter
Show

Sleep 700ms
Type "bastion-session explain vmordws02 -o json"
Enter
Sleep 1400ms
Type "bastion-session session renew vmordws02 -o json"
Enter
Sleep 1300ms
Type "bastion-session ssh-config audit vmordws02 -o json"
Enter
Sleep 1800ms
