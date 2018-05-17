make
if [ $? -eq 0 ]; then
    ./iwikii
else
    echo Build failed. Aborting.
fi