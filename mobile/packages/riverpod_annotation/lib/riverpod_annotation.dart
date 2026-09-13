/// Annotation used by Riverpod feature providers.
///
/// The generated provider declarations are checked in under each feature's
/// `providers/*.g.dart` file so the app can build without a network-only code
/// generation step.
class Riverpod {
  const Riverpod({this.keepAlive = false});

  final bool keepAlive;
}

const riverpod = Riverpod();
