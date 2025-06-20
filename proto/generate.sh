cd proto
buf generate --template buf.gen.gogo.yaml
buf generate --template buf.gen.pulsar.yaml
cd ..

cp -r github.com/provlabs/vault/* ./
cp -r api/vault/* api/
find api/ -type f -name "*.go" -exec sed -i 's|github.com/provlabs/vault/api/vault|github.com/provlabs/vault/api|g' {} +

rm -rf github.com
rm -rf api/vault
rm -rf vault
