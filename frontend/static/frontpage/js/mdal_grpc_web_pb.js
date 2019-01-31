/**
 * @fileoverview gRPC-Web generated client stub for mdalgrpc
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!



const grpc = {};
grpc.web = require('grpc-web');

const proto = {};
proto.mdalgrpc = require('./mdal_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?Object} options
 * @constructor
 * @struct
 * @final
 */
proto.mdalgrpc.MDALClient =
    function(hostname, credentials, options) {
  if (!options) options = {};
  options['format'] = 'text';

  /**
   * @private @const {!grpc.web.GrpcWebClientBase} The client
   */
  this.client_ = new grpc.web.GrpcWebClientBase(options);

  /**
   * @private @const {string} The hostname
   */
  this.hostname_ = hostname;

  /**
   * @private @const {?Object} The credentials to be used to connect
   *    to the server
   */
  this.credentials_ = credentials;

  /**
   * @private @const {?Object} Options for the client
   */
  this.options_ = options;
};


/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?Object} options
 * @constructor
 * @struct
 * @final
 */
proto.mdalgrpc.MDALPromiseClient =
    function(hostname, credentials, options) {
  if (!options) options = {};
  options['format'] = 'text';

  /**
   * @private @const {!proto.mdalgrpc.MDALClient} The delegate callback based client
   */
  this.delegateClient_ = new proto.mdalgrpc.MDALClient(
      hostname, credentials, options);

};


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.mdalgrpc.DataQueryRequest,
 *   !proto.mdalgrpc.DataQueryResponse>}
 */
const methodInfo_DataQuery = new grpc.web.AbstractClientBase.MethodInfo(
  proto.mdalgrpc.DataQueryResponse,
  /** @param {!proto.mdalgrpc.DataQueryRequest} request */
  function(request) {
    return request.serializeBinary();
  },
  proto.mdalgrpc.DataQueryResponse.deserializeBinary
);


/**
 * @param {!proto.mdalgrpc.DataQueryRequest} request The request proto
 * @param {!Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.mdalgrpc.DataQueryResponse>}
 *     The XHR Node Readable Stream
 */
proto.mdalgrpc.MDALClient.prototype.dataQuery =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/mdalgrpc.MDAL/DataQuery',
      request,
      metadata,
      methodInfo_DataQuery);
};


module.exports = proto.mdalgrpc;

