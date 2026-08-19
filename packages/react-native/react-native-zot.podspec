Pod::Spec.new do |s|
  s.name = "react-native-zot"
  s.version = "0.0.1"
  s.summary = "Zot agent runtime for React Native"
  s.homepage = "https://github.com/patriceckhart/zot-native-sdk"
  s.license = { :type => "MIT" }
  s.author = "Patrice Eckhart"
  s.platforms = { :ios => "15.0", :osx => "12.0" }
  s.source = { :git => s.homepage + ".git", :tag => s.version }
  s.source_files = "ios/**/*.{swift,m,h}"
  s.vendored_frameworks = "ios/Zot.xcframework"
  s.dependency "React-Core"
  s.swift_version = "5.9"
end
