# Indexation nocturne sur la machine d'indexation (compte de service avec
# droits d'écriture sur \\SERVEUR\EBM\.index). À adapter.
$exe  = "C:\EBM-Indexer\indexer.exe"
$args = '-root "\\SERVEUR\EBM" -pdftotext "C:\poppler\Library\bin\pdftotext.exe" -workers 8'
$action  = New-ScheduledTaskAction -Execute $exe -Argument $args -WorkingDirectory "C:\EBM-Indexer"
$trigger = New-ScheduledTaskTrigger -Daily -At 2am
Register-ScheduledTask -TaskName "EBM Archive - Indexation" -Action $action -Trigger $trigger `
  -User "DOMAINE\svc-ebm-index" -Password (Read-Host "Mot de passe du compte de service") -RunLevel Limited
