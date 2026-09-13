Pod::Spec.new do |s|
  s.name             = 'file_picker'
  s.version          = '1.0.0'
  s.summary          = 'Minimal local CSV file picker.'
  s.description      = 'Minimal local CSV file picker for Spendable Today.'
  s.homepage         = 'https://localhost'
  s.license          = { :type => 'MIT' }
  s.author           = { 'Spendable Today' => 'local@example.invalid' }
  s.source           = { :path => '.' }
  s.source_files     = 'Classes/**/*'
  s.dependency 'Flutter'
  s.platform = :ios, '13.0'
  s.swift_version = '5.0'
end

