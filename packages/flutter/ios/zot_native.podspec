Pod::Spec.new do |s|
  s.name = "zot_native"
  s.version = "0.0.1"
  s.summary = "Zot agent runtime for Flutter"
  s.homepage = "https://github.com/patriceckhart/zot-native-sdk"
  s.license = { :type => "MIT" }
  s.author = "Patrice Eckhart"
  s.source = { :path => "." }
  s.source_files = "Classes/**/*"
  s.vendored_frameworks = "zot_native/Zot.xcframework"
  s.platform = :ios, "15.0"
  s.dependency "Flutter"
  s.swift_version = "5.9"
end
