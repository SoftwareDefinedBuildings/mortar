# Generated by the gRPC Python protocol compiler plugin. DO NOT EDIT!
import grpc

import mortar_pb2 as mortar__pb2


class MortarStub(object):
  # missing associated documentation comment in .proto file
  pass

  def __init__(self, channel):
    """Constructor.

    Args:
      channel: A grpc.Channel.
    """
    self.GetAPIKey = channel.unary_unary(
        '/mortar.Mortar/GetAPIKey',
        request_serializer=mortar__pb2.GetAPIKeyRequest.SerializeToString,
        response_deserializer=mortar__pb2.APIKeyResponse.FromString,
        )
    self.Qualify = channel.unary_unary(
        '/mortar.Mortar/Qualify',
        request_serializer=mortar__pb2.QualifyRequest.SerializeToString,
        response_deserializer=mortar__pb2.QualifyResponse.FromString,
        )
    self.Fetch = channel.unary_stream(
        '/mortar.Mortar/Fetch',
        request_serializer=mortar__pb2.FetchRequest.SerializeToString,
        response_deserializer=mortar__pb2.FetchResponse.FromString,
        )


class MortarServicer(object):
  # missing associated documentation comment in .proto file
  pass

  def GetAPIKey(self, request, context):
    # missing associated documentation comment in .proto file
    pass
    context.set_code(grpc.StatusCode.UNIMPLEMENTED)
    context.set_details('Method not implemented!')
    raise NotImplementedError('Method not implemented!')

  def Qualify(self, request, context):
    """identify which sites meet the requirements of the queries
    """
    context.set_code(grpc.StatusCode.UNIMPLEMENTED)
    context.set_details('Method not implemented!')
    raise NotImplementedError('Method not implemented!')

  def Fetch(self, request, context):
    """pull data from Mortar
    """
    context.set_code(grpc.StatusCode.UNIMPLEMENTED)
    context.set_details('Method not implemented!')
    raise NotImplementedError('Method not implemented!')


def add_MortarServicer_to_server(servicer, server):
  rpc_method_handlers = {
      'GetAPIKey': grpc.unary_unary_rpc_method_handler(
          servicer.GetAPIKey,
          request_deserializer=mortar__pb2.GetAPIKeyRequest.FromString,
          response_serializer=mortar__pb2.APIKeyResponse.SerializeToString,
      ),
      'Qualify': grpc.unary_unary_rpc_method_handler(
          servicer.Qualify,
          request_deserializer=mortar__pb2.QualifyRequest.FromString,
          response_serializer=mortar__pb2.QualifyResponse.SerializeToString,
      ),
      'Fetch': grpc.unary_stream_rpc_method_handler(
          servicer.Fetch,
          request_deserializer=mortar__pb2.FetchRequest.FromString,
          response_serializer=mortar__pb2.FetchResponse.SerializeToString,
      ),
  }
  generic_handler = grpc.method_handlers_generic_handler(
      'mortar.Mortar', rpc_method_handlers)
  server.add_generic_rpc_handlers((generic_handler,))
