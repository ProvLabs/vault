cd proto
buf generate --template buf.gen.gogo.yaml
buf generate --template buf.gen.pulsar.yaml
cd ..

cp -r github.com/provlabs/vault/* ./

# Remove temp files from generation
# TODO: have these generate into a temp dir instead then remove that dir
rm -rf github.com
rm -rf provlabs
